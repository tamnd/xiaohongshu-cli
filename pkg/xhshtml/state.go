// Package xhshtml extracts the embedded application state from Xiaohongshu's
// server-rendered pages. Each page carries a window.__INITIAL_STATE__ object
// that holds the same records the JSON API serves, which is what lets the
// anonymous surfaces work without a login cookie.
package xhshtml

import (
	"bytes"
	"encoding/json"
	"errors"
)

var marker = []byte("window.__INITIAL_STATE__")

// ErrNotFound is returned when a page carries no __INITIAL_STATE__ object. In
// practice this means the request was redirected to a login or block page.
var ErrNotFound = errors.New("xhshtml: __INITIAL_STATE__ not found")

// State returns the window.__INITIAL_STATE__ object embedded in an Xiaohongshu
// page as valid JSON. The site assigns the object with bare `undefined` and
// `NaN` tokens that are not valid JSON, so those are rewritten to null outside
// of string literals before the object is returned.
func State(html []byte) (json.RawMessage, error) {
	i := bytes.Index(html, marker)
	if i < 0 {
		return nil, ErrNotFound
	}
	start := i + len(marker)
	for start < len(html) && (html[start] == ' ' || html[start] == '=') {
		start++
	}
	if start >= len(html) || html[start] != '{' {
		return nil, ErrNotFound
	}
	obj, err := scanObject(html[start:])
	if err != nil {
		return nil, err
	}
	return json.RawMessage(sanitize(obj)), nil
}

// scanObject returns the bytes of the first complete JSON object at the start of
// b, matching braces while respecting string literals and escapes.
func scanObject(b []byte) ([]byte, error) {
	depth := 0
	inStr := false
	esc := false
	for i := range len(b) {
		ch := b[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case ch == '\\':
				esc = true
			case ch == '"':
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return b[:i+1], nil
			}
		}
	}
	return nil, errors.New("xhshtml: unbalanced state object")
}

// sanitize rewrites the bare JS tokens undefined, NaN, and Infinity to null when
// they appear outside string literals, leaving string contents untouched.
func sanitize(b []byte) []byte {
	tokens := []string{"undefined", "NaN", "Infinity", "-Infinity"}
	out := make([]byte, 0, len(b))
	inStr := false
	esc := false
	for i := 0; i < len(b); i++ {
		ch := b[i]
		if inStr {
			out = append(out, ch)
			switch {
			case esc:
				esc = false
			case ch == '\\':
				esc = true
			case ch == '"':
				inStr = false
			}
			continue
		}
		if ch == '"' {
			inStr = true
			out = append(out, ch)
			continue
		}
		matched := false
		for _, t := range tokens {
			if hasToken(b[i:], t) {
				out = append(out, []byte("null")...)
				i += len(t) - 1
				matched = true
				break
			}
		}
		if !matched {
			out = append(out, ch)
		}
	}
	return out
}

func hasToken(b []byte, t string) bool {
	if len(b) < len(t) {
		return false
	}
	return string(b[:len(t)]) == t
}
