package xiaohongshu

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// newTestClient builds a client with the pacing turned off, so the retry tests
// measure the policy rather than the clock.
func newTestClient(t *testing.T, cfg Config) *Client {
	t.Helper()
	cfg.Rate = 0
	c := NewClient(cfg)
	c.SetNow(time.Now)
	return c
}

// TestRetryOnlyRepeatsTheTransport is the whole point of the retry rework. In
// v0.2.0 every error that was not an *APIError was retried, so a dead endpoint
// cost four requests and reported itself as a network failure.
func TestRetryOnlyRepeatsTheTransport(t *testing.T) {
	cases := []struct {
		name  string
		err   error
		calls int
	}{
		{"a dead endpoint is asked once", statusError(StatusGone, 404, "", "/x"), 1},
		{"a login wall is asked once", statusError(StatusLogin, -101, "", "/x"), 1},
		{"a missing token is asked once", statusError(StatusToken, 300031, "", "/x"), 1},
		{"a refused signature is asked once", statusError(StatusAntibot, 406, "", "/x"), 1},
		{"a walled address is asked once", statusError(StatusWalled, -510, "", "/x"), 1},
		{"a missing note is asked once", statusError(StatusNotFound, -1, "", "/x"), 1},
		{"the transport is asked four times", errors.New("dial tcp: connection reset"), 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cl := newTestClient(t, Config{Retries: 3, NoCache: true})
			calls := 0
			_, err := cl.runWithRetry(context.Background(), "k", false, func(context.Context) (result, error) {
				calls++
				return result{}, c.err
			})
			if err == nil {
				t.Fatal("want an error")
			}
			if calls != c.calls {
				t.Fatalf("attempted %d times, want %d", calls, c.calls)
			}
		})
	}
}

func TestRetryReportsHowManyAttemptsItMade(t *testing.T) {
	cl := newTestClient(t, Config{Retries: 2, NoCache: true})
	_, err := cl.runWithRetry(context.Background(), "k", false, func(context.Context) (result, error) {
		return result{}, errors.New("timeout")
	})
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatalf("want an *APIError, got %T", err)
	}
	if ae.Hint != "gave up after 3 attempts" {
		t.Fatalf("hint = %q, want the attempt count", ae.Hint)
	}
	if ae.Status != StatusNetwork {
		t.Fatalf("status = %s, want network", ae.Status)
	}
}

// TestRetriesZeroMeansOneRequest guards the off-by-one that made --retries 0
// still say "request failed after retries".
func TestRetriesZeroMeansOneRequest(t *testing.T) {
	cl := newTestClient(t, Config{Retries: 0, NoCache: true})
	calls := 0
	_, err := cl.runWithRetry(context.Background(), "k", false, func(context.Context) (result, error) {
		calls++
		return result{}, errors.New("timeout")
	})
	if calls != 1 {
		t.Fatalf("attempted %d times, want 1", calls)
	}
	var ae *APIError
	if !errors.As(err, &ae) || ae.Hint != "gave up after 1 attempts" {
		t.Fatalf("hint = %v, want the attempt count", err)
	}
}

func TestRetryRecoversWhenTheTransportSettles(t *testing.T) {
	cl := newTestClient(t, Config{Retries: 3, NoCache: true})
	calls := 0
	body, err := cl.runWithRetry(context.Background(), "k", false, func(context.Context) (result, error) {
		calls++
		if calls < 3 {
			return result{}, errors.New("connection reset")
		}
		return result{body: []byte(`{"code":0}`), status: StatusOK}, nil
	})
	if err != nil {
		t.Fatalf("want success on the third try, got %v", err)
	}
	if string(body) != `{"code":0}` {
		t.Fatalf("body = %q", body)
	}
	if calls != 3 {
		t.Fatalf("attempted %d times, want 3", calls)
	}
}

// TestCacheKeepsAnswersAndNotRefusals is the second half of the same fix. A
// refusal describes the caller's standing at a moment, so keeping one poisons
// every later run from the same directory.
func TestCacheKeepsAnswersAndNotRefusals(t *testing.T) {
	cases := []struct {
		name   string
		status Status
		kept   bool
	}{
		{"ok", StatusOK, true},
		{"empty", StatusEmpty, true},
		{"notfound", StatusNotFound, true},
		{"login", StatusLogin, false},
		{"walled", StatusWalled, false},
		{"token", StatusToken, false},
		{"antibot", StatusAntibot, false},
		// A removed endpoint is not cached, so verify can notice when the site
		// puts it back rather than reading last week's answer.
		{"gone", StatusGone, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			cl := newTestClient(t, Config{Retries: 0, CacheDir: dir, CacheTTL: time.Hour})
			if cl.cache == nil {
				t.Fatal("want a cache")
			}
			_, err := cl.runWithRetry(context.Background(), "k", true, func(context.Context) (result, error) {
				return result{body: []byte(`{"code":0}`), status: c.status}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			_, hit := cl.cache.get("k")
			if hit != c.kept {
				t.Fatalf("cached = %v, want %v", hit, c.kept)
			}
			if entries, _ := os.ReadDir(dir); (len(entries) > 0) != c.kept {
				t.Fatalf("cache directory has %d entries, want kept=%v", len(entries), c.kept)
			}
		})
	}
}

func TestCacheIsReadBeforeAnyRequest(t *testing.T) {
	dir := t.TempDir()
	cl := newTestClient(t, Config{Retries: 0, CacheDir: dir, CacheTTL: time.Hour})
	cl.cache.put("k", []byte(`{"cached":true}`))
	calls := 0
	body, err := cl.runWithRetry(context.Background(), "k", true, func(context.Context) (result, error) {
		calls++
		return result{body: []byte(`{"fresh":true}`), status: StatusOK}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("made %d requests, want 0", calls)
	}
	if string(body) != `{"cached":true}` {
		t.Fatalf("body = %q, want the cached one", body)
	}
}
