package cli

import (
	"bytes"
	"strings"
	"testing"
)

type sample struct {
	NoteID string `json:"note_id"`
	Title  string `json:"title"`
	Likes  int64  `json:"liked_count"`
	URL    string `json:"url"`
}

func render(t *testing.T, format Format, fields []string, recs ...any) string {
	t.Helper()
	var buf bytes.Buffer
	o, err := NewOutput(&buf, format, fields, false, "")
	if err != nil {
		t.Fatalf("NewOutput: %v", err)
	}
	for _, r := range recs {
		if err := o.Emit(r); err != nil {
			t.Fatalf("Emit: %v", err)
		}
	}
	if err := o.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return buf.String()
}

func TestJSONLOneRecordPerLine(t *testing.T) {
	out := render(t, FormatJSONL, nil,
		sample{NoteID: "n1", Title: "a", Likes: 1},
		sample{NoteID: "n2", Title: "b", Likes: 2},
	)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), out)
	}
	if !strings.Contains(lines[0], `"note_id":"n1"`) {
		t.Errorf("line 0 = %q", lines[0])
	}
}

func TestJSONArray(t *testing.T) {
	out := render(t, FormatJSON, nil, sample{NoteID: "n1"})
	if !strings.HasPrefix(strings.TrimSpace(out), "[") || !strings.HasSuffix(strings.TrimSpace(out), "]") {
		t.Fatalf("json output is not an array: %q", out)
	}
}

func TestJSONEmptyIsEmptyArray(t *testing.T) {
	out := strings.TrimSpace(render(t, FormatJSON, nil))
	if out != "[]" {
		t.Fatalf("empty json = %q, want []", out)
	}
}

func TestCSVHeaderAndFields(t *testing.T) {
	out := render(t, FormatCSV, []string{"note_id", "title"}, sample{NoteID: "n1", Title: "hi", Likes: 9})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if lines[0] != "note_id,title" {
		t.Fatalf("header = %q, want note_id,title", lines[0])
	}
	if lines[1] != "n1,hi" {
		t.Fatalf("row = %q, want n1,hi", lines[1])
	}
}

func TestURLFormatPicksURLField(t *testing.T) {
	out := strings.TrimSpace(render(t, FormatURL, nil, sample{NoteID: "n1", URL: "https://x/n1"}))
	if out != "https://x/n1" {
		t.Fatalf("url output = %q, want the url field", out)
	}
}

func TestTSVUsesTabs(t *testing.T) {
	out := render(t, FormatTSV, []string{"note_id", "title"}, sample{NoteID: "n1", Title: "hi"})
	if !strings.Contains(out, "n1\thi") {
		t.Fatalf("tsv row not tab-separated: %q", out)
	}
}
