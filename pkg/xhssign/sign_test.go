package xhssign

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fixedSigner returns a Signer with a frozen clock and deterministic bytes so
// the output is reproducible.
func fixedSigner(tsMillis int64) *Signer {
	n := byte(0)
	return &Signer{
		NowMillis: func() int64 { return tsMillis },
		Rand: func(k int) []byte {
			b := make([]byte, k)
			for i := range b {
				n++
				b[i] = n
			}
			return b
		},
	}
}

func TestSignXSShape(t *testing.T) {
	s := fixedSigner(1700000000000)
	h := s.Sign(Request{
		Method:  "GET",
		URI:     "/api/sns/web/v1/user_posted",
		Params:  map[string]string{"num": "30", "user_id": "abc"},
		A1:      "19abcdef0000000000000000000000000000000050000123",
		Cookies: map[string]string{"a1": "19abcdef", "webId": "deadbeef"},
	})
	if !strings.HasPrefix(h.XS, "XYW_") {
		t.Fatalf("x-s missing XYW_ prefix: %q", h.XS)
	}
	if h.XT != "1700000000000" {
		t.Fatalf("x-t = %q, want 1700000000000", h.XT)
	}
	// The base64 after the prefix must decode to a JSON doc with the expected keys.
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(h.XS, "XYW_"))
	if err != nil {
		t.Fatalf("x-s body not base64: %v", err)
	}
	var doc map[string]string
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("x-s body not json: %v", err)
	}
	for _, k := range []string{"signSvn", "signType", "appId", "signVersion", "payload"} {
		if doc[k] == "" {
			t.Errorf("x-s doc missing %q", k)
		}
	}
	if doc["appId"] != "xhs-pc-web" {
		t.Errorf("appId = %q", doc["appId"])
	}
	if doc["signType"] != "x2" {
		t.Errorf("signType = %q, want x2", doc["signType"])
	}
}

func TestSignDeterministic(t *testing.T) {
	req := Request{
		Method:  "POST",
		URI:     "/api/sns/web/v1/feed",
		Body:    `{"source_note_id":"x"}`,
		A1:      "19abc",
		Cookies: map[string]string{"a1": "19abc"},
	}
	a := fixedSigner(1700000000000).Sign(req)
	b := fixedSigner(1700000000000).Sign(req)
	if a.XS != b.XS {
		t.Errorf("x-s not deterministic:\n%s\n%s", a.XS, b.XS)
	}
	if a.XSCommon != b.XSCommon {
		t.Errorf("x-s-common not deterministic")
	}
}

func TestXSCommonDecodes(t *testing.T) {
	s := fixedSigner(1700000000000)
	h := s.Sign(Request{Method: "GET", URI: "/x", A1: "19abc", Cookies: map[string]string{"a1": "19abc"}})
	raw, err := customB64.DecodeString(h.XSCommon)
	if err != nil {
		t.Fatalf("x-s-common not custom base64: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("x-s-common not json: %v (%s)", err, raw)
	}
	if string(doc["x5"]) != `"19abc"` {
		t.Errorf("x5 = %s, want a1", doc["x5"])
	}
	if _, ok := doc["x9"]; !ok {
		t.Error("x-s-common missing x9")
	}
}

func TestContentString(t *testing.T) {
	got := ContentString("GET", "/api/x", map[string]string{"b": "2", "a": "1"}, "")
	if got != "/api/x?a=1&b=2" {
		t.Errorf("GET content = %q", got)
	}
	got = ContentString("POST", "/api/x", nil, `{"k":"v"}`)
	if got != `/api/x{"k":"v"}` {
		t.Errorf("POST content = %q", got)
	}
	got = ContentString("GET", "/api/x", nil, "")
	if got != "/api/x" {
		t.Errorf("empty GET content = %q", got)
	}
}

func TestCRC32JSVector(t *testing.T) {
	// Known: crc32_ieee("") == 0, so crc32JS("") == int32(uint32(0xEDB88320)).
	var poly uint32 = 0xEDB88320
	want := int32(poly)
	if got := crc32JS(""); got != want {
		t.Errorf("crc32JS(\"\") = %d, want %d", got, want)
	}
}

func TestCustomB64RoundTrip(t *testing.T) {
	in := []byte("the quick brown fox jumps over 12345")
	enc := customB64.EncodeToString(in)
	dec, err := customB64.DecodeString(enc)
	if err != nil {
		t.Fatal(err)
	}
	if string(dec) != string(in) {
		t.Errorf("round trip: %q != %q", dec, in)
	}
}

func TestGenerateA1Shape(t *testing.T) {
	a1 := GenerateA1(time.UnixMilli(1700000000000), func(n int) []byte {
		b := make([]byte, n)
		for i := range b {
			b[i] = byte(i)
		}
		return b
	})
	if len(a1) != 52 {
		t.Errorf("a1 len = %d, want 52 (%q)", len(a1), a1)
	}
	if WebID(a1) == "" || len(WebID(a1)) != 32 {
		t.Errorf("web id len = %d", len(WebID(a1)))
	}
}

func TestXrayTraceLength(t *testing.T) {
	s := fixedSigner(1700000000000)
	h := s.Sign(Request{Method: "GET", URI: "/x", A1: "a", Cookies: map[string]string{"a1": "a"}})
	if len(h.XrayTrace) != 32 {
		t.Errorf("xray len = %d (%q)", len(h.XrayTrace), h.XrayTrace)
	}
	if len(h.B3TraceID) != 16 {
		t.Errorf("b3 len = %d", len(h.B3TraceID))
	}
	_ = strconv.Itoa
}
