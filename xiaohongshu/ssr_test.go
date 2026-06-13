package xiaohongshu

import (
	"encoding/json"
	"testing"
)

func TestHumanCount(t *testing.T) {
	cases := map[string]int64{
		"":      0,
		"123":   123,
		"1.9万":  19000,
		"10万+":  100000,
		"2亿":    200000000,
		"3.5w":  35000,
		"12K":   12000,
		" 88 ":  88,
		"not":   0,
		"1.2万+": 12000,
	}
	for in, want := range cases {
		if got := humanCount(in); got != want {
			t.Errorf("humanCount(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", "x", "y"); got != "x" {
		t.Errorf("firstNonEmpty = %q, want x", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Errorf("firstNonEmpty empty = %q, want empty", got)
	}
}

func TestSSRUserBriefName(t *testing.T) {
	if (ssrUserBrief{Nickname: "a"}).name() != "a" {
		t.Error("nickname not preferred")
	}
	if (ssrUserBrief{NickName: "b"}).name() != "b" {
		t.Error("nickName fallback missing")
	}
}

// TestConvertSSRNote checks that a server-rendered note decodes through the
// camelCase shapes and the human-count parser into a clean Note, including the
// video stream codecs that the web page nests under media.stream.
func TestConvertSSRNote(t *testing.T) {
	const raw = `{
	  "noteId": "n1",
	  "type": "video",
	  "title": "hello",
	  "desc": "a body",
	  "time": 1700000000000,
	  "ipLocation": "Tokyo",
	  "user": {"userId": "u1", "nickName": "bob", "avatar": "av"},
	  "xsecToken": "tok2",
	  "interactInfo": {"likedCount": "1.9万", "collectedCount": "10万+", "commentCount": "12", "shareCount": "3"},
	  "imageList": [{"urlDefault": "https://img/1.jpg", "width": 100, "height": 200}],
	  "video": {"capa": {"duration": 42}, "media": {"video": {"md5": "abc"}, "stream": {"h264": [{"masterUrl": "https://v/h264.mp4", "width": 720, "height": 1280}], "h265": [{"masterUrl": "https://v/h265.mp4"}]}}},
	  "tagList": [{"name": "coffee"}],
	  "atUserList": [{"nickname": "alice"}]
	}`
	var r ssrNote
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("decode: %v", err)
	}
	n := convertSSRNote(r, "seed", fixedNow())
	if n.NoteID != "n1" || n.Type != "video" || n.Title != "hello" {
		t.Fatalf("scalars: %+v", n)
	}
	if n.Nickname != "bob" {
		t.Errorf("nickName not read: %q", n.Nickname)
	}
	if n.LikedCount != 19000 || n.CollectedCount != 100000 {
		t.Errorf("human counts: liked=%d collected=%d", n.LikedCount, n.CollectedCount)
	}
	if n.XsecToken != "tok2" {
		t.Errorf("token: note token should win, got %q", n.XsecToken)
	}
	if len(n.Images) != 1 || n.Images[0].URL != "https://img/1.jpg" {
		t.Errorf("images: %+v", n.Images)
	}
	if n.Video == nil || n.Video.Duration != 42 || n.Video.MD5 != "abc" {
		t.Fatalf("video: %+v", n.Video)
	}
	if len(n.Video.Masters) != 2 {
		t.Errorf("video masters = %d, want 2 (h264+h265)", len(n.Video.Masters))
	}
	if n.Video.Width != 720 || n.Video.Height != 1280 {
		t.Errorf("video dims = %dx%d, want 720x1280", n.Video.Width, n.Video.Height)
	}
	if len(n.Tags) != 1 || n.Tags[0] != "coffee" {
		t.Errorf("tags: %+v", n.Tags)
	}
	if len(n.AtUsers) != 1 || n.AtUsers[0] != "alice" {
		t.Errorf("at users: %+v", n.AtUsers)
	}
}

// TestSSRNoteState checks the wrapper that holds note detail under the keyed map.
func TestSSRNoteState(t *testing.T) {
	const raw = `{"note": {"noteDetailMap": {"n1": {"note": {"noteId": "n1", "title": "t"}}}}}`
	var s ssrNoteState
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	entry, ok := s.Note.NoteDetailMap["n1"]
	if !ok || entry.Note.Title != "t" {
		t.Fatalf("note detail map: %+v", s)
	}
}

// TestSSRExploreState checks the explore feed shape and the feed-item flattener.
func TestSSRExploreState(t *testing.T) {
	const raw = `{"feed": {"feeds": [
	  {"id": "n1", "xsecToken": "tok", "noteCard": {"type": "normal", "displayTitle": "hi", "user": {"userId": "u1", "nickName": "bob"}, "cover": {"urlDefault": "https://c/1.jpg"}, "interactInfo": {"likedCount": "2.1万"}}}
	]}}`
	var s ssrExploreState
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(s.Feed.Feeds) != 1 {
		t.Fatalf("feeds = %d", len(s.Feed.Feeds))
	}
	c := NewClient(Config{})
	c.SetNow(fixedNow)
	fi := c.feedItem(s.Feed.Feeds[0])
	if fi.NoteID != "n1" || fi.Title != "hi" || fi.Nickname != "bob" {
		t.Fatalf("feed item: %+v", fi)
	}
	if fi.LikedCount != 21000 {
		t.Errorf("liked = %d, want 21000", fi.LikedCount)
	}
	if fi.Cover != "https://c/1.jpg" {
		t.Errorf("cover = %q", fi.Cover)
	}
	if fi.XsecToken != "tok" {
		t.Errorf("token = %q", fi.XsecToken)
	}
}

// TestSSRUserState checks the profile shape, the loose gender field, and the
// note grid that user-notes reads anonymously.
func TestSSRUserState(t *testing.T) {
	const raw = `{"user": {"userPageData": {
	  "basicInfo": {"nickname": "carol", "redId": "123", "desc": "hello", "gender": "1", "ipLocation": "Osaka"},
	  "interactions": [{"type": "fans", "count": "5.2万"}, {"type": "follows", "count": "88"}],
	  "tags": [{"name": "travel"}]
	}, "notes": [[
	  {"id": "n9", "xsecToken": "tk", "noteCard": {"displayTitle": "trip", "user": {"userId": "u2", "nickName": "carol"}}}
	]]}}`
	var s ssrUserState
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	d := s.User.UserPageData
	if d.BasicInfo.Nickname != "carol" || d.BasicInfo.RedID != "123" {
		t.Fatalf("basic info: %+v", d.BasicInfo)
	}
	if int(d.BasicInfo.Gender) != 1 {
		t.Errorf("loose gender = %d, want 1", int(d.BasicInfo.Gender))
	}
	if len(s.User.Notes) != 1 || len(s.User.Notes[0]) != 1 || s.User.Notes[0][0].ID != "n9" {
		t.Fatalf("note grid: %+v", s.User.Notes)
	}
}

// TestLooseInt covers the number, quoted-number, and non-numeric forms the
// profile gender field has taken across pages.
func TestLooseInt(t *testing.T) {
	var n looseInt
	for _, in := range []string{`1`, `"1"`, `"unknown"`, `null`, `""`} {
		if err := n.UnmarshalJSON([]byte(in)); err != nil {
			t.Errorf("looseInt(%s) errored: %v", in, err)
		}
	}
	n = 0
	_ = n.UnmarshalJSON([]byte(`"2"`))
	if int(n) != 2 {
		t.Errorf("looseInt quoted = %d, want 2", int(n))
	}
}
