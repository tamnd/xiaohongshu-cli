package xiaohongshu

import (
	"testing"
	"time"
)

func fixedNow() time.Time { return time.Unix(1700000000, 0).UTC() }

func TestConvertNoteImage(t *testing.T) {
	r := rawNote{
		NoteID: "n1",
		Type:   "normal",
		Title:  "hi",
		User:   rawNoteUser{UserID: "u1", Nickname: "bob"},
	}
	r.InteractInfo.LikedCount = "123"
	r.ImageList = append(r.ImageList, struct {
		URLDefault string `json:"url_default"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		TraceID    string `json:"trace_id"`
		LivePhoto  bool   `json:"live_photo"`
		Stream     struct {
			H264 []struct {
				MasterURL string `json:"master_url"`
			} `json:"h264"`
		} `json:"stream"`
	}{URLDefault: "https://img/1.jpg", Width: 100, Height: 200})

	n := convertNote(r, "tok", fixedNow())
	if n.NoteID != "n1" || n.LikedCount != 123 {
		t.Fatalf("bad scalars: %+v", n)
	}
	if n.XsecToken != "tok" {
		t.Fatalf("token not carried: %q", n.XsecToken)
	}
	if len(n.Images) != 1 || n.Images[0].URL != "https://img/1.jpg" {
		t.Fatalf("image not converted: %+v", n.Images)
	}
	if n.URL != "https://www.xiaohongshu.com/explore/n1" {
		t.Fatalf("url = %q", n.URL)
	}
}

func TestAtoi(t *testing.T) {
	cases := map[string]int64{"": 0, "5": 5, "x": 0, "1000": 1000}
	for in, want := range cases {
		if got := atoi(in); got != want {
			t.Errorf("atoi(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestFeedCategoriesNonEmpty(t *testing.T) {
	cats := FeedCategories()
	if len(cats) == 0 {
		t.Fatal("no feed categories")
	}
	found := false
	for _, c := range cats {
		if c == "recommend" {
			found = true
		}
	}
	if !found {
		t.Error("recommend category missing")
	}
}

func TestSearchIDStable(t *testing.T) {
	a := searchID("coffee")
	b := searchID("coffee")
	if a != b {
		t.Fatalf("searchID not stable: %q != %q", a, b)
	}
	if len(a) != 21 {
		t.Fatalf("searchID length = %d, want 21", len(a))
	}
	if searchID("coffee") == searchID("tea") {
		t.Error("searchID should vary by keyword")
	}
}
