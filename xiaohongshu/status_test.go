package xiaohongshu

import (
	"net/http"
	"net/url"
	"testing"
)

// allStates is every state the classifier can produce. The predicate tests
// iterate it rather than listing cases, so adding a state to the vocabulary
// without deciding what it means for caching or retrying fails the build.
var allStates = []Status{
	StatusOK, StatusEmpty, StatusLogin, StatusWalled, StatusToken,
	StatusNotFound, StatusGone, StatusAntibot, StatusNetwork, StatusError,
}

func TestStatusStringIsDefinedForEveryState(t *testing.T) {
	seen := map[string]Status{}
	for _, s := range allStates {
		name, ok := statusNames[s]
		if !ok || name == "" {
			t.Errorf("state %d has no name", int(s))
			continue
		}
		if prev, dup := seen[name]; dup {
			t.Errorf("states %d and %d share the name %q", int(prev), int(s), name)
		}
		seen[name] = s
		if got := s.String(); got != name {
			t.Errorf("String() = %q, want %q", got, name)
		}
	}
	if got := Status(99).String(); got != "status(99)" {
		t.Errorf("unknown state: want status(99), got %q", got)
	}
}

func TestStatusPredicates(t *testing.T) {
	want := map[Status]struct{ refused, cacheable, retryable bool }{
		StatusOK:       {false, true, false},
		StatusEmpty:    {false, true, false},
		StatusLogin:    {true, false, false},
		StatusWalled:   {true, false, false},
		StatusToken:    {true, false, false},
		StatusNotFound: {false, true, false},
		StatusGone:     {true, false, false},
		StatusAntibot:  {true, false, false},
		StatusNetwork:  {false, false, true},
		StatusError:    {false, false, false},
	}
	for _, s := range allStates {
		w, ok := want[s]
		if !ok {
			t.Fatalf("state %s has no expectation, add one", s)
		}
		if got := s.Refused(); got != w.refused {
			t.Errorf("%s.Refused() = %v, want %v", s, got, w.refused)
		}
		if got := s.Cacheable(); got != w.cacheable {
			t.Errorf("%s.Cacheable() = %v, want %v", s, got, w.cacheable)
		}
		if got := s.Retryable(); got != w.retryable {
			t.Errorf("%s.Retryable() = %v, want %v", s, got, w.retryable)
		}
	}
}

func TestCodeStatus(t *testing.T) {
	cases := []struct {
		code int
		want Status
		ok   bool
	}{
		{0, StatusOK, true},
		{-1, StatusAntibot, true},
		{-100, StatusLogin, true},
		{-101, StatusLogin, true},
		{-510, StatusWalled, true},
		{10013, StatusWalled, true},
		{300012, StatusAntibot, true},
		{300013, StatusAntibot, true},
		{300015, StatusAntibot, true},
		{300031, StatusToken, true},
		{406, StatusAntibot, true},
		{461, StatusAntibot, true},
		{7777, StatusError, false},
	}
	for _, c := range cases {
		got, ok := codeStatus(c.code)
		if got != c.want || ok != c.ok {
			t.Errorf("codeStatus(%d) = %s,%v; want %s,%v", c.code, got, ok, c.want, c.ok)
		}
	}
}

func TestMessageStatus(t *testing.T) {
	for _, msg := range []string{"笔记不存在", "该笔记不存在或已删除", "用户不存在"} {
		if got, ok := messageStatus(msg); !ok || got != StatusNotFound {
			t.Errorf("messageStatus(%q) = %s,%v; want notfound,true", msg, got, ok)
		}
	}
	if got, ok := messageStatus("当前笔记暂时无法浏览"); ok {
		t.Errorf("messageStatus of an unlisted message = %s,%v; want error,false", got, ok)
	}
}

func TestClassifyAPI(t *testing.T) {
	cases := []struct {
		name string
		code int
		body string
		want Status
	}{
		{"ok", 200, `{"success":true,"code":0,"data":{"items":[1]}}`, StatusOK},
		{"empty object", 200, `{"success":true,"code":0,"data":{}}`, StatusEmpty},
		{"empty null", 200, `{"success":true,"code":0,"data":null}`, StatusEmpty},
		{"empty list is an answer", 200, `{"success":true,"code":0,"data":[]}`, StatusOK},
		{"login", 200, `{"success":false,"code":-101,"msg":"无登录信息，或登录信息为空"}`, StatusLogin},
		{"token", 200, `{"success":false,"code":300031,"msg":"当前笔记暂时无法浏览"}`, StatusToken},
		{"walled", 200, `{"success":false,"code":-510,"msg":""}`, StatusWalled},
		{"note gone by message", 200, `{"success":false,"code":-1,"msg":"笔记不存在"}`, StatusNotFound},
		// The router answers a dead path before the signature gate, with Go's
		// own plain-text body, which is why this is not JSON.
		{"dead endpoint", 404, "404 page not found\n", StatusGone},
		{"signature refused", 406, `{"code":-1,"success":false}`, StatusAntibot},
		{"gateway 461", 461, ``, StatusAntibot},
		{"too many requests", 429, ``, StatusWalled},
		{"server error", 503, ``, StatusNetwork},
		{"redirect body", 302, ``, StatusError},
		{"unparseable", 200, `<!doctype html>`, StatusError},
		{"unknown code", 200, `{"success":false,"code":7777,"msg":"?"}`, StatusError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := classifyAPI(c.code, []byte(c.body)); got != c.want {
				t.Errorf("classifyAPI(%d, %q) = %s, want %s", c.code, c.body, got, c.want)
			}
		})
	}
}

func TestClassifyWeb(t *testing.T) {
	cases := []struct {
		name     string
		code     int
		hasState bool
		want     Status
	}{
		{"rendered", 200, true, StatusOK},
		// Some surfaces are served with a 404 status and a full body, so the
		// state object outranks the code.
		{"rendered under a 404", 404, true, StatusOK},
		{"missing", 404, false, StatusNotFound},
		{"shell", 200, false, StatusEmpty},
		{"walled", 429, false, StatusWalled},
		{"server error", 500, true, StatusNetwork},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := classifyWeb(c.code, c.hasState); got != c.want {
				t.Errorf("classifyWeb(%d, %v) = %s, want %s", c.code, c.hasState, got, c.want)
			}
		})
	}
}

// TestStopAtRefusal covers the three real Location headers. The token refusal
// carries a redirectPath parameter of its own, which is exactly why the previous
// version reported a note missing its xsec_token as an account problem.
func TestStopAtRefusal(t *testing.T) {
	cases := []struct {
		name    string
		loc     string
		stopped bool
		kind    refusalKind
	}{
		{
			name:    "login wall",
			loc:     "https://www.xiaohongshu.com/login?redirectPath=https%3A%2F%2Fwww.xiaohongshu.com%2Fexplore&exSource=",
			stopped: true,
			kind:    refusalLogin,
		},
		{
			name:    "missing token",
			loc:     "https://www.xiaohongshu.com/404/sec_ab12cd?redirectPath=%2Fexplore&error_code=300031&error_msg=%E5%BD%93%E5%89%8D%E7%AC%94%E8%AE%B0%E6%9A%82%E6%97%B6%E6%97%A0%E6%B3%95%E6%B5%8F%E8%A7%88&uuid=x&verifyMsg=",
			stopped: true,
			kind:    refusalToken,
		},
		{
			name:    "an ordinary redirect is followed",
			loc:     "https://www.xiaohongshu.com/explore/68a1b2c3",
			stopped: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u, err := url.Parse(c.loc)
			if err != nil {
				t.Fatal(err)
			}
			err = stopAtRefusal(&http.Request{URL: u}, nil)
			if !c.stopped {
				if err != nil {
					t.Fatalf("want the redirect followed, got %v", err)
				}
				return
			}
			ref, ok := err.(*refusal)
			if !ok {
				t.Fatalf("want a *refusal, got %T (%v)", err, err)
			}
			if ref.kind != c.kind {
				t.Fatalf("kind = %d, want %d", ref.kind, c.kind)
			}
			if got := classifyRefusal(ref); c.kind == refusalToken && got != StatusToken {
				t.Fatalf("classifyRefusal = %s, want token", got)
			}
		})
	}
}

func TestRefusalCarriesTheQueryString(t *testing.T) {
	u, err := url.Parse("https://www.xiaohongshu.com/404/sec_ab12cd?error_code=300031&error_msg=" +
		"%E5%BD%93%E5%89%8D%E7%AC%94%E8%AE%B0%E6%9A%82%E6%97%B6%E6%97%A0%E6%B3%95%E6%B5%8F%E8%A7%88")
	if err != nil {
		t.Fatal(err)
	}
	ref := &refusal{kind: refusalToken, at: u}
	if got := ref.errorCode(); got != 300031 {
		t.Errorf("errorCode = %d, want 300031", got)
	}
	if got := ref.errorMessage(); got != "当前笔记暂时无法浏览" {
		t.Errorf("errorMessage = %q", got)
	}
	// A login refusal has neither, and asking must not panic.
	login := &refusal{kind: refusalLogin, at: &url.URL{Path: "/login"}}
	if got := login.errorCode(); got != 0 {
		t.Errorf("login errorCode = %d, want 0", got)
	}
	if got := login.errorMessage(); got != "" {
		t.Errorf("login errorMessage = %q, want empty", got)
	}
}

func TestIsEmptyPayload(t *testing.T) {
	cases := map[string]bool{
		``:            true,
		`null`:        true,
		`{}`:          true,
		`  {}  `:      true,
		`[]`:          false,
		`{"a":1}`:     false,
		`[{"a":1}]`:   false,
		`"something"`: false,
	}
	for in, want := range cases {
		if got := isEmptyPayload([]byte(in)); got != want {
			t.Errorf("isEmptyPayload(%q) = %v, want %v", in, got, want)
		}
	}
}
