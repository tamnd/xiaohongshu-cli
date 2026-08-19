package xiaohongshu

import (
	"errors"
	"fmt"
	"strings"
)

// APIError is a refusal from Xiaohongshu, sorted into a state and given an
// English hint that says what to do about it.
type APIError struct {
	Code     int    // the envelope code, or the HTTP status where there was no envelope
	Message  string // upstream message, usually Chinese, kept as it was sent
	Hint     string // English, and it names one cause rather than listing two
	Status   Status
	Endpoint string // the path that answered, because the surfaces move
}

// Error reads as one sentence: what the site said, what it means, and which
// request it was.
//
// The endpoint goes last and in brackets rather than first, because the first
// thing a reader needs is the sentence, and because a leading path turns the
// whole message into something the terminal's error styling title-cases into
// nonsense. It is printed whenever it is known, since the first question
// anybody asks about a refusal is which request it was.
func (e *APIError) Error() string {
	var b strings.Builder
	switch {
	case e.Code != 0:
		fmt.Fprintf(&b, "xiaohongshu %d: ", e.Code)
	case e.Status != StatusOK:
		fmt.Fprintf(&b, "%s: ", e.Status)
	}
	switch {
	case e.Hint != "" && e.Message != "":
		fmt.Fprintf(&b, "%s (%s)", e.Hint, e.Message)
	case e.Hint != "":
		b.WriteString(e.Hint)
	default:
		b.WriteString(e.Message)
	}
	if e.Endpoint != "" {
		fmt.Fprintf(&b, " [%s]", e.Endpoint)
	}
	return b.String()
}

// hints are the sentences the tool says when it refuses. Each answers what
// happened, on which surface, and what changes it. None of them joins two
// causes with "or": where the tool cannot tell two situations apart it probes,
// and where it cannot probe it says which one it is assuming and why.
var hints = map[Status]string{
	StatusLogin:  "this surface needs a logged-in cookie, so pass one with --cookie or set XHS_COOKIE",
	StatusWalled: "this address is being rate limited by xiaohongshu, so wait several minutes and try again",
	StatusToken: "this note needs an xsec_token and none was supplied or the one supplied was refused, " +
		"so get the note from `xhs feed` or `xhs search`, which carry a token on every item, or paste the full share URL",
	StatusAntibot: "the request signature was refused by xiaohongshu, which no cookie fixes, so run `xhs verify --live` " +
		"to see whether the signing scheme has moved",
	StatusGone:     "this endpoint was removed by xiaohongshu and asking again will not bring it back",
	StatusNotFound: "not found, or removed",
	StatusEmpty:    "the surface answered and had nothing to give",
	StatusNetwork:  "the request did not complete",
}

// statusError builds the error for a state, with the upstream code and message
// attached where there were any.
func statusError(s Status, code int, message, endpoint string) *APIError {
	return &APIError{
		Code:     code,
		Message:  message,
		Hint:     hints[s],
		Status:   s,
		Endpoint: endpoint,
	}
}

// apiError maps a code and message from a JSON envelope into a typed error.
func apiError(code int, message string) *APIError {
	s, ok := codeStatus(code)
	if !ok {
		s = StatusError
	}
	if ms, hit := messageStatus(message); hit {
		s = ms
	}
	return statusError(s, code, message, "")
}

// StatusOf reports the state an error came from, so a program embedding this
// library can branch on the same distinction the exit codes expose.
//
// It unwraps, because a refusal that a caller has annotated on the way up is
// still a refusal, and reading it as a plain error would put it back in the
// retry loop it was classified to stay out of. Anything with no state in it is
// the transport, which is the one thing worth asking again about.
func StatusOf(err error) Status {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae.Status
	}
	if err != nil {
		return StatusNetwork
	}
	return StatusOK
}
