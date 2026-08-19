package xiaohongshu

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Status is what a response turned out to be, once the transport, the host, the
// redirect target, the envelope and the payload have all had their say.
//
// Xiaohongshu is the mirror image of an API that says yes when it means no.
// Nothing here masquerades as success. The problem is the opposite one: almost
// everything the caller cannot have arrives looking like everything else the
// caller cannot have, and four different situations produce the same HTTP 302
// to the same login page. Telling them apart is what this type exists for.
type Status int

// The eight response states, plus the two that describe having no response to
// classify at all.
const (
	StatusOK       Status = iota // a payload arrived and it has something in it
	StatusEmpty                  // the handler ran, was allowed, and had nothing
	StatusLogin                  // this surface needs a logged-in cookie
	StatusWalled                 // this address is over budget, waiting is the fix
	StatusToken                  // the object needs an xsec_token we do not hold
	StatusNotFound               // the object does not exist
	StatusGone                   // the endpoint itself is gone
	StatusAntibot                // the signature or the fingerprint was refused
	StatusNetwork                // transport, timeout, DNS
	StatusError                  // classified as nothing, which is a bug report
)

var statusNames = map[Status]string{
	StatusOK:       "ok",
	StatusEmpty:    "empty",
	StatusLogin:    "login",
	StatusWalled:   "walled",
	StatusToken:    "token",
	StatusNotFound: "notfound",
	StatusGone:     "gone",
	StatusAntibot:  "antibot",
	StatusNetwork:  "network",
	StatusError:    "error",
}

func (s Status) String() string {
	if n, ok := statusNames[s]; ok {
		return n
	}
	return "status(" + strconv.Itoa(int(s)) + ")"
}

// Refused reports whether the caller was prevented from learning the answer, as
// opposed to learning that the answer is nothing. An empty list is a claim
// about the world; a refusal is a claim about the caller.
func (s Status) Refused() bool {
	switch s {
	case StatusLogin, StatusWalled, StatusToken, StatusAntibot, StatusGone:
		return true
	}
	return false
}

// Cacheable reports whether this response is worth keeping.
//
// A refusal never is: it describes the caller's standing at a moment, not the
// object they asked for, and caching one poisons every later run from the same
// directory. StatusNotFound is cacheable and StatusGone is not, which looks
// inconsistent and is not. A note that does not exist will still not exist in
// an hour, whereas a removed endpoint might be restored, and a cached
// StatusGone would hide exactly the drift the verify command exists to catch.
func (s Status) Cacheable() bool {
	switch s {
	case StatusOK, StatusEmpty, StatusNotFound:
		return true
	}
	return false
}

// Retryable reports whether asking again, soon, could plausibly answer
// differently.
//
// Only the transport qualifies, plus the two HTTP conditions handled before
// classification: a 429 and a 5xx are a server saying it is busy. Everything
// else is policy, routing, or a capability check. A 404 is the routing table, a
// 302 to /login is the caller's budget, and a 406 is the signature. None of the
// three is different two seconds later, and retrying the second one is how a
// rate-limited address stays rate-limited.
func (s Status) Retryable() bool { return s == StatusNetwork }

// refusalKind is which of the two redirect destinations a request was stopped
// at. They are told apart by path, not by query: the token refusal carries a
// redirectPath parameter too, so keying on the query reads it as a login wall.
type refusalKind int

const (
	refusalLogin refusalKind = iota // /login?redirectPath=...
	refusalToken                    // /404/sec_<rand>?error_code=300031&...
)

// refusal is the typed value the redirect hook returns instead of an error
// carrying a magic substring. The URL is the point: the token refusal puts its
// reason in the query string, and stringifying the error throws that away.
type refusal struct {
	kind refusalKind
	at   *url.URL
}

func (r *refusal) Error() string {
	if r.kind == refusalToken {
		return "redirected to the token refusal page: " + r.at.String()
	}
	return "redirected to the login page: " + r.at.String()
}

// errorCode returns the error_code the token refusal page carries, or 0.
func (r *refusal) errorCode() int {
	if r == nil || r.at == nil {
		return 0
	}
	n, err := strconv.Atoi(r.at.Query().Get("error_code"))
	if err != nil {
		return 0
	}
	return n
}

// errorMessage returns the error_msg the token refusal page carries. It is
// Chinese and it is the site's own words, so it travels with the diagnosis
// rather than being translated away.
func (r *refusal) errorMessage() string {
	if r == nil || r.at == nil {
		return ""
	}
	return r.at.Query().Get("error_msg")
}

// stopAtRefusal stops the HTTP client from following a redirect to either
// refusal page, so the caller sees the wall rather than a rendered login form
// with no data in it.
//
// Order matters. Both destinations carry a redirectPath parameter, so a check
// on the query alone reports a token refusal as a login wall, which is what the
// previous version did and why a note fetched without its xsec_token was
// reported as an account problem.
func stopAtRefusal(req *http.Request, _ []*http.Request) error {
	switch {
	case strings.HasPrefix(req.URL.Path, "/login"):
		return &refusal{kind: refusalLogin, at: req.URL}
	case strings.HasPrefix(req.URL.Path, "/404/sec_"):
		return &refusal{kind: refusalToken, at: req.URL}
	}
	return nil
}

// codeStatus maps an envelope code to a state. Measured against the live API
// except where noted.
func codeStatus(code int) (Status, bool) {
	switch code {
	case 0:
		return StatusOK, true
	case -1:
		// Paired with an HTTP 406 from the gateway. The body is
		// {"code":-1,"success":false} and says nothing else.
		return StatusAntibot, true
	case -100:
		return StatusLogin, true
	case -101:
		// 无登录信息，或登录信息为空. The handler ran and wants web_session.
		return StatusLogin, true
	case -510, 10013:
		return StatusWalled, true
	case 300012, 300013, 300015:
		return StatusAntibot, true
	case 300031:
		// 当前笔记暂时无法浏览, which is what a note without a usable
		// xsec_token answers. It is also what an id that is not a note
		// answers, so this state cannot distinguish the two.
		return StatusToken, true
	case 406, 461:
		return StatusAntibot, true
	}
	return StatusError, false
}

// messageStatus recognises the messages the site sends with a generic code.
// It runs after the code table so a specific code always wins.
func messageStatus(msg string) (Status, bool) {
	switch msg {
	case "笔记不存在", "该笔记不存在或已删除", "用户不存在":
		return StatusNotFound, true
	}
	return StatusError, false
}

// classifyAPI sorts one JSON API response into a state.
//
// Cheapest signal first, stopping at the first one that decides: the stopped
// redirect, then the HTTP status, then the content type, then the envelope
// code and message, and only then the payload rule.
func classifyAPI(statusCode int, body []byte) Status {
	switch {
	case statusCode == http.StatusNotFound:
		// The router answers this before the signature gate, with Go's own
		// plain-text http.NotFound body. A real content 404 would be JSON.
		return StatusGone
	case statusCode == 406 || statusCode == 461:
		return StatusAntibot
	case statusCode == http.StatusTooManyRequests:
		return StatusWalled
	case statusCode >= 500:
		return StatusNetwork
	case statusCode < 200 || statusCode >= 300:
		return StatusError
	}

	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return StatusError
	}
	if s, ok := messageStatus(env.Msg); ok && env.Code != 0 {
		return s
	}
	s, ok := codeStatus(env.Code)
	if !ok {
		return StatusError
	}
	if s != StatusOK {
		return s
	}
	if isEmptyPayload(env.Data) {
		return StatusEmpty
	}
	return StatusOK
}

// classifyWeb sorts one server-rendered response into a state. hasState says
// whether __INITIAL_STATE__ was found in the body.
func classifyWeb(statusCode int, hasState bool) Status {
	switch {
	case statusCode == http.StatusTooManyRequests:
		return StatusWalled
	case statusCode >= 500:
		return StatusNetwork
	}
	// Some surfaces are served with a 404 status and a fully rendered body, so
	// the presence of the state object matters more than the code does.
	if hasState {
		return StatusOK
	}
	if statusCode == http.StatusNotFound {
		return StatusNotFound
	}
	// A 200 with no state object is a shell. It is not a refusal, because a
	// refusal arrives as a redirect and never gets this far.
	return StatusEmpty
}

// classifyRefusal turns a stopped redirect into a state. A login redirect is
// ambiguous on its own, because the site serves it both to a surface that wants
// a credential and to an address that has spent its budget; the caller resolves
// that with a probe.
func classifyRefusal(r *refusal) Status {
	if r.kind == refusalToken {
		return StatusToken
	}
	return StatusLogin
}

// isEmptyPayload reports whether a code 0 response carried nothing.
//
// null and {} are nothing. [] is deliberately not: an empty array is a real
// answer that says the collection has no members, whereas an absent object is
// the handler declining to fill one in.
func isEmptyPayload(data json.RawMessage) bool {
	s := strings.TrimSpace(string(data))
	return s == "" || s == "null" || s == "{}"
}
