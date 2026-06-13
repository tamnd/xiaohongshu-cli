// Package xhsurl parses Xiaohongshu ids and URLs. It pulls the note id, user
// id, and the xsec_token out of the many link shapes the site uses, so the
// commands can accept a raw id, a full share URL, or a clipboard paste.
package xhsurl

import (
	"net/url"
	"regexp"
	"strings"
)

// Kind is the sort of thing a reference points at.
type Kind int

const (
	KindUnknown Kind = iota
	KindNote
	KindUser
)

// Ref is a parsed reference.
type Ref struct {
	Kind      Kind
	NoteID    string
	UserID    string
	XsecToken string
	// XsecSource is the source tag that travels with the token (pc_feed, pc_search).
	XsecSource string
}

// hexID matches the 24-character hex object ids XHS uses for notes and users.
var hexID = regexp.MustCompile(`^[0-9a-fA-F]{24}$`)

// idInPath matches a 24-hex id sitting in a URL path segment.
var idInPath = regexp.MustCompile(`[0-9a-fA-F]{24}`)

// Parse turns a raw id or URL into a Ref. A bare 24-hex id is treated as a note
// id by default; callers that expect a user id should use ParseUser.
func Parse(s string) Ref {
	s = strings.TrimSpace(s)
	if s == "" {
		return Ref{}
	}
	if hexID.MatchString(s) {
		return Ref{Kind: KindNote, NoteID: s}
	}
	if strings.Contains(s, "xiaohongshu.com") || strings.HasPrefix(s, "http") {
		return parseURL(s)
	}
	// A non-URL, non-hex string is taken as an id as-is (some user ids are short).
	return Ref{Kind: KindNote, NoteID: s}
}

// ParseUser is Parse but treats a bare id as a user id.
func ParseUser(s string) Ref {
	r := Parse(s)
	if r.Kind == KindNote && r.UserID == "" && !strings.Contains(s, "explore") && !strings.Contains(s, "discovery") {
		// A bare id passed to a user command is a user id.
		if hexID.MatchString(strings.TrimSpace(s)) || !strings.HasPrefix(s, "http") {
			return Ref{Kind: KindUser, UserID: r.NoteID, XsecToken: r.XsecToken, XsecSource: r.XsecSource}
		}
	}
	return r
}

func parseURL(s string) Ref {
	u, err := url.Parse(s)
	if err != nil {
		if m := idInPath.FindString(s); m != "" {
			return Ref{Kind: KindNote, NoteID: m}
		}
		return Ref{}
	}
	q := u.Query()
	ref := Ref{
		XsecToken:  q.Get("xsec_token"),
		XsecSource: q.Get("xsec_source"),
	}
	path := strings.Trim(u.Path, "/")
	segs := strings.Split(path, "/")
	for i, seg := range segs {
		switch seg {
		case "explore", "item":
			if i+1 < len(segs) && segs[i+1] != "" {
				ref.Kind = KindNote
				ref.NoteID = segs[i+1]
				return ref
			}
		case "profile":
			if i+1 < len(segs) && segs[i+1] != "" {
				ref.Kind = KindUser
				ref.UserID = segs[i+1]
				return ref
			}
		}
	}
	// Fall back to the first id-shaped path segment.
	if m := idInPath.FindString(path); m != "" {
		ref.Kind = KindNote
		ref.NoteID = m
	}
	return ref
}

// NoteURL builds the canonical explore URL for a note, carrying its token.
func NoteURL(noteID, xsecToken string) string {
	u := "https://www.xiaohongshu.com/explore/" + noteID
	if xsecToken != "" {
		u += "?xsec_token=" + xsecToken + "&xsec_source=pc_feed"
	}
	return u
}

// UserURL builds the canonical profile URL for a user.
func UserURL(userID string) string {
	return "https://www.xiaohongshu.com/user/profile/" + userID
}
