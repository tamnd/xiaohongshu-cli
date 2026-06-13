package xhsurl

import "testing"

func TestParseNoteURL(t *testing.T) {
	r := Parse("https://www.xiaohongshu.com/explore/65a1b2c3d4e5f6a7b8c9d0e1?xsec_token=ABC123&xsec_source=pc_feed")
	if r.Kind != KindNote {
		t.Fatalf("kind = %v", r.Kind)
	}
	if r.NoteID != "65a1b2c3d4e5f6a7b8c9d0e1" {
		t.Errorf("note id = %q", r.NoteID)
	}
	if r.XsecToken != "ABC123" {
		t.Errorf("token = %q", r.XsecToken)
	}
}

func TestParseDiscoveryItem(t *testing.T) {
	r := Parse("https://www.xiaohongshu.com/discovery/item/65a1b2c3d4e5f6a7b8c9d0e1")
	if r.Kind != KindNote || r.NoteID != "65a1b2c3d4e5f6a7b8c9d0e1" {
		t.Errorf("got %+v", r)
	}
}

func TestParseUserProfile(t *testing.T) {
	r := Parse("https://www.xiaohongshu.com/user/profile/5ff0000000000000010203")
	if r.Kind != KindUser {
		t.Fatalf("kind = %v (%+v)", r.Kind, r)
	}
	if r.UserID != "5ff0000000000000010203" {
		t.Errorf("user id = %q", r.UserID)
	}
}

func TestParseBareHex(t *testing.T) {
	r := Parse("65a1b2c3d4e5f6a7b8c9d0e1")
	if r.Kind != KindNote || r.NoteID != "65a1b2c3d4e5f6a7b8c9d0e1" {
		t.Errorf("got %+v", r)
	}
}

func TestParseUserBare(t *testing.T) {
	r := ParseUser("5ff0000000000000010203456789abcd")
	if r.Kind != KindUser {
		t.Errorf("ParseUser kind = %v (%+v)", r.Kind, r)
	}
}

func TestNoteURL(t *testing.T) {
	got := NoteURL("abc", "tok")
	want := "https://www.xiaohongshu.com/explore/abc?xsec_token=tok&xsec_source=pc_feed"
	if got != want {
		t.Errorf("NoteURL = %q", got)
	}
}
