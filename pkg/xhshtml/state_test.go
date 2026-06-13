package xhshtml

import (
	"encoding/json"
	"testing"
)

func TestStateBasic(t *testing.T) {
	html := []byte(`<html><body><script>window.__INITIAL_STATE__={"a":1,"b":undefined,"c":"keep undefined here","d":NaN}</script></body></html>`)
	st, err := State(html)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(st, &m); err != nil {
		t.Fatalf("result not valid JSON: %v\n%s", err, st)
	}
	if m["a"].(float64) != 1 {
		t.Errorf("a = %v", m["a"])
	}
	if m["b"] != nil {
		t.Errorf("undefined not nulled: %v", m["b"])
	}
	if m["c"] != "keep undefined here" {
		t.Errorf("string content corrupted: %q", m["c"])
	}
	if m["d"] != nil {
		t.Errorf("NaN not nulled: %v", m["d"])
	}
}

func TestStateNested(t *testing.T) {
	html := []byte(`x=1;window.__INITIAL_STATE__ = {"feed":{"feeds":[{"id":"n1","noteCard":{"title":"hi {brace} \" quote"}}]}};</script>`)
	st, err := State(html)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	var s struct {
		Feed struct {
			Feeds []struct {
				ID       string `json:"id"`
				NoteCard struct {
					Title string `json:"title"`
				} `json:"noteCard"`
			} `json:"feeds"`
		} `json:"feed"`
	}
	if err := json.Unmarshal(st, &s); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, st)
	}
	if len(s.Feed.Feeds) != 1 || s.Feed.Feeds[0].ID != "n1" {
		t.Fatalf("nested parse failed: %+v", s)
	}
	if s.Feed.Feeds[0].NoteCard.Title != `hi {brace} " quote` {
		t.Errorf("title with braces/quotes mishandled: %q", s.Feed.Feeds[0].NoteCard.Title)
	}
}

func TestStateMissing(t *testing.T) {
	if _, err := State([]byte(`<html>login wall</html>`)); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
