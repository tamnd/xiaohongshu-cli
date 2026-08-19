package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/tamnd/xiaohongshu-cli/xiaohongshu"
)

// allStates is every state the library can report. Adding one without giving it
// an exit code fails here rather than silently landing in the catch-all.
var allStates = []xiaohongshu.Status{
	xiaohongshu.StatusOK,
	xiaohongshu.StatusEmpty,
	xiaohongshu.StatusLogin,
	xiaohongshu.StatusWalled,
	xiaohongshu.StatusToken,
	xiaohongshu.StatusNotFound,
	xiaohongshu.StatusGone,
	xiaohongshu.StatusAntibot,
	xiaohongshu.StatusNetwork,
	xiaohongshu.StatusError,
}

func TestEveryStateHasItsOwnExitCode(t *testing.T) {
	seen := map[int]xiaohongshu.Status{}
	for _, s := range allStates {
		code, ok := exitCodes[s]
		if !ok {
			t.Errorf("state %s has no exit code", s)
			continue
		}
		if prev, dup := seen[code]; dup {
			t.Errorf("states %s and %s both exit %d, which is what v0.3.0 exists to stop", prev, s, code)
		}
		seen[code] = s
	}
	if len(exitCodes) != len(allStates) {
		t.Errorf("the table has %d entries for %d states", len(exitCodes), len(allStates))
	}
}

func TestExitCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"success", nil, 0},
		{"a plain error is the transport", errors.New("dial tcp: i/o timeout"), 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := exitCode(c.err); got != c.want {
				t.Errorf("exitCode(%v) = %d, want %d", c.err, got, c.want)
			}
		})
	}

	// Every classified refusal reaches its own number, including through a
	// wrapper, because commands annotate errors on the way up.
	for _, s := range allStates {
		if s == xiaohongshu.StatusOK {
			continue
		}
		err := &xiaohongshu.APIError{Status: s, Endpoint: "/x"}
		if got, want := exitCode(err), exitCodes[s]; got != want {
			t.Errorf("exitCode(%s) = %d, want %d", s, got, want)
		}
		wrapped := fmt.Errorf("fetch note: %w", err)
		if got, want := exitCode(wrapped), exitCodes[s]; got != want {
			t.Errorf("exitCode(wrapped %s) = %d, want %d", s, got, want)
		}
	}
}

// TestTheCodesAScriptDependsOnDoNotMove pins the five numbers shared with
// bilibili-cli, so one wrapper can drive both tools.
func TestTheCodesAScriptDependsOnDoNotMove(t *testing.T) {
	shared := map[xiaohongshu.Status]int{
		xiaohongshu.StatusAntibot:  2,
		xiaohongshu.StatusEmpty:    3,
		xiaohongshu.StatusNetwork:  5,
		xiaohongshu.StatusWalled:   6,
		xiaohongshu.StatusNotFound: 7,
	}
	for s, want := range shared {
		if got := exitCodes[s]; got != want {
			t.Errorf("%s exits %d, want %d to match bilibili-cli", s, got, want)
		}
	}
}
