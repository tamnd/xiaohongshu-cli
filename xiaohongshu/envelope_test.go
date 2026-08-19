package xiaohongshu

import (
	"encoding/json"
	"testing"
)

func TestEnvelopeMiss(t *testing.T) {
	var e Envelope
	e.miss("collected_count", "login")
	e.miss("liked_count", "login")
	if len(e.Missed) != 2 {
		t.Fatalf("missed = %v, want two entries", e.Missed)
	}
	if e.Missed["collected_count"] != "login" {
		t.Fatalf("missed[collected_count] = %q", e.Missed["collected_count"])
	}
	// The second call must not have replaced the map built by the first.
	if _, ok := e.Missed["liked_count"]; !ok {
		t.Fatal("the second miss dropped the first")
	}
}

// TestEnvelopeCloneIsDeep matters because one reading can stand behind several
// records, and a shallow copy would let a miss recorded on the second record
// appear on the first.
func TestEnvelopeCloneIsDeep(t *testing.T) {
	a := Envelope{Endpoint: "/api/sns/web/v1/user/otherinfo", Host: "api", Signed: true, Status: "ok"}
	a.miss("collected_count", "login")

	b := a.clone()
	b.miss("liked_count", "login")
	b.Endpoint = "/explore"

	if len(a.Missed) != 1 {
		t.Fatalf("the clone wrote back into the original: %v", a.Missed)
	}
	if a.Endpoint != "/api/sns/web/v1/user/otherinfo" {
		t.Fatalf("endpoint = %q", a.Endpoint)
	}
	if len(b.Missed) != 2 {
		t.Fatalf("clone missed = %v, want two entries", b.Missed)
	}

	// A clone of an envelope that missed nothing still must not share a map.
	empty := Envelope{}.clone()
	if empty.Missed != nil {
		t.Fatalf("missed = %v, want nil", empty.Missed)
	}
}

func TestEnvelopeMarksApproximateCounts(t *testing.T) {
	var e Envelope
	if e.Approx {
		t.Fatal("a fresh envelope claims approximate counts")
	}
	e.markApprox()
	if !e.Approx {
		t.Fatal("markApprox did nothing")
	}
}

// TestEnvelopeOmitsWhatItHasNothingToSayAbout keeps the common record small:
// approx and missed only appear when they carry information.
func TestEnvelopeOmitsWhatItHasNothingToSayAbout(t *testing.T) {
	b, err := json.Marshal(Envelope{
		Endpoint: "/explore/68a1b2c3",
		Host:     "web",
		Status:   "ok",
		Fetched:  "2026-08-17T10:04:05Z",
		Bytes:    989624,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	want := `{"endpoint":"/explore/68a1b2c3","host":"web","signed":false,"status":"ok",` +
		`"fetched":"2026-08-17T10:04:05Z","bytes":989624}`
	if got != want {
		t.Fatalf("json = %s\nwant %s", got, want)
	}
}
