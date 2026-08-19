package xiaohongshu

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
)

// fixture is a round tripper that answers every request the same way. It is the
// stand-in for an httptest server, which cannot be used here because it binds a
// socket and every test binary has to pass with the network denied.
type fixture struct {
	status int
	body   string
	calls  int
}

func (f *fixture) RoundTrip(req *http.Request) (*http.Response, error) {
	f.calls++
	return &http.Response{
		StatusCode: f.status,
		Body:       io.NopCloser(bytes.NewReader([]byte(f.body))),
		Header:     http.Header{},
		Request:    req,
	}, nil
}

func answering(t *testing.T, status int, body string) (*Client, *fixture) {
	t.Helper()
	f := &fixture{status: status, body: body}
	c := NewClient(Config{NoCache: true, Retries: 0, Rate: 0})
	c.SetTransport(f)
	return c, f
}

func TestMe(t *testing.T) {
	cases := []struct {
		name string
		body string
		want Me
	}{
		{
			name: "logged in",
			body: `{"success":true,"code":0,"data":{"guest":false,"user_id":"5f3a1b","nickname":"咖啡日记","red_id":"12345678"}}`,
			want: Me{LoggedIn: true, UserID: "5f3a1b", Nickname: "咖啡日记", RedID: "12345678"},
		},
		{
			// A guest flag with an id attached is still a guest. That id belongs
			// to the device rather than to an account.
			name: "guest with a device id",
			body: `{"success":true,"code":0,"data":{"guest":true,"user_id":"5f3a1b"}}`,
			want: Me{LoggedIn: false, UserID: "5f3a1b"},
		},
		{
			name: "no id at all",
			body: `{"success":true,"code":0,"data":{"guest":false}}`,
			want: Me{LoggedIn: false},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cl, _ := answering(t, 200, c.body)
			got, err := cl.Me(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

// TestMeAnswersWithoutACookie is the fix for the one command that exists to
// diagnose a missing cookie failing whenever the cookie was missing.
//
// Everywhere else in the tool a -101 means the caller asked for something they
// cannot have. Here the question is whether they are logged in, and "you are
// not" is a complete and correct reply.
func TestMeAnswersWithoutACookie(t *testing.T) {
	cl, f := answering(t, 200, `{"success":false,"code":-101,"msg":"无登录信息，或登录信息为空"}`)
	me, err := cl.Me(context.Background())
	if err != nil {
		t.Fatalf("want a clean answer, got %v", err)
	}
	if me.LoggedIn {
		t.Error("logged_in is true with no cookie")
	}
	if f.calls != 1 {
		t.Errorf("made %d requests, want 1", f.calls)
	}
}

// TestMeStillFailsOnEverythingElse keeps the exemption narrow. A refused
// signature is not an answer to "am I logged in", and reporting it as one would
// tell the caller their cookie is missing when their cookie is fine.
func TestMeStillFailsOnEverythingElse(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   Status
	}{
		{"signature refused", 406, `{"code":-1,"success":false}`, StatusAntibot},
		{"walled", 200, `{"success":false,"code":-510,"msg":""}`, StatusWalled},
		{"endpoint gone", 404, "404 page not found\n", StatusGone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cl, _ := answering(t, c.status, c.body)
			if _, err := cl.Me(context.Background()); err == nil {
				t.Fatal("want an error")
			} else if got := StatusOf(err); got != c.want {
				t.Fatalf("status = %s, want %s", got, c.want)
			}
		})
	}
}
