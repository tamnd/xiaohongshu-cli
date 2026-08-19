package xiaohongshu

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestAPIErrorMessage(t *testing.T) {
	cases := []struct {
		name string
		err  *APIError
		want string
	}{
		{
			name: "code, hint, message and endpoint",
			err: &APIError{
				Code: 300031, Message: "当前笔记暂时无法浏览", Hint: "needs a token",
				Status: StatusToken, Endpoint: "/explore/68a1b2c3",
			},
			want: "xiaohongshu 300031: needs a token (当前笔记暂时无法浏览) [/explore/68a1b2c3]",
		},
		{
			// A stopped redirect has no envelope, so the state names itself.
			name: "no code",
			err:  &APIError{Hint: "needs a cookie", Status: StatusLogin, Endpoint: "/explore"},
			want: "login: needs a cookie [/explore]",
		},
		{
			name: "no endpoint",
			err:  &APIError{Message: "connection reset", Hint: "gave up after 4 attempts", Status: StatusNetwork},
			want: "network: gave up after 4 attempts (connection reset)",
		},
		{
			name: "message only",
			err:  &APIError{Code: 7777, Message: "?"},
			want: "xiaohongshu 7777: ?",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.err.Error(); got != c.want {
				t.Errorf("Error() = %q\nwant       %q", got, c.want)
			}
		})
	}
}

func TestEveryRefusalCarriesAHint(t *testing.T) {
	for _, s := range allStates {
		if s == StatusOK || s == StatusError {
			continue
		}
		if hints[s] == "" {
			t.Errorf("state %s refuses with no hint about what to do", s)
		}
	}
}

// TestNoHintOffersTwoCauses guards the rule the release is built on: where the
// tool cannot tell two situations apart it probes, and where it cannot probe it
// says which one it is assuming. It never joins two causes with "or".
func TestNoHintOffersTwoCauses(t *testing.T) {
	for s, h := range hints {
		for _, bad := range []string{" or the site", "may be", "might be", "possibly", "probably"} {
			if strings.Contains(h, bad) {
				t.Errorf("hint for %s hedges with %q: %s", s, bad, h)
			}
		}
	}
}

func TestAPIErrorFromEnvelope(t *testing.T) {
	cases := []struct {
		code int
		msg  string
		want Status
	}{
		{-101, "无登录信息，或登录信息为空", StatusLogin},
		{300031, "当前笔记暂时无法浏览", StatusToken},
		{-510, "", StatusWalled},
		// The message outranks a generic code, which is how a deleted note is
		// told apart from everything else code -1 covers.
		{-1, "笔记不存在", StatusNotFound},
		{-1, "", StatusAntibot},
		{7777, "who knows", StatusError},
	}
	for _, c := range cases {
		err := apiError(c.code, c.msg)
		if err.Status != c.want {
			t.Errorf("apiError(%d, %q).Status = %s, want %s", c.code, c.msg, err.Status, c.want)
		}
		if err.Code != c.code {
			t.Errorf("apiError(%d).Code = %d", c.code, err.Code)
		}
	}
}

func TestStatusOf(t *testing.T) {
	if got := StatusOf(nil); got != StatusOK {
		t.Errorf("StatusOf(nil) = %s, want ok", got)
	}
	// Anything that is not a classified refusal is the transport, which is the
	// only thing worth asking again about.
	if got := StatusOf(errors.New("dial tcp: i/o timeout")); got != StatusNetwork {
		t.Errorf("StatusOf(plain) = %s, want network", got)
	}
	if got := StatusOf(statusError(StatusGone, 404, "", "/x")); got != StatusGone {
		t.Errorf("StatusOf(*APIError) = %s, want gone", got)
	}
	// A wrapped refusal must not be read as a network failure and retried.
	wrapped := fmt.Errorf("fetch note: %w", statusError(StatusToken, 300031, "", "/x"))
	if got := StatusOf(wrapped); got != StatusToken {
		t.Errorf("StatusOf(wrapped) = %s, want token", got)
	}
}
