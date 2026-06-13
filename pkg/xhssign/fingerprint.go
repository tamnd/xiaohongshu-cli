package xhssign

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"hash/crc32"
	"strconv"
	"strings"
	"time"
)

// fingerprintB1 builds the JSON object that b1 encrypts. It is the subset of
// the web client's device fingerprint that feeds b1 (the x33..x82 keys). The
// values are fixed and plausible for a desktop Chrome on Windows. They do not
// need to be unique per run; they only need to form a well-shaped object the
// server can decode.
func fingerprintB1(cookies map[string]string) string {
	cookieStr := cookieString(cookies)
	_ = cookieStr // reserved: some fields embed the cookie string; b1's subset does not.
	fields := []struct{ k, v string }{
		{"x33", `"0"`},
		{"x34", `"0"`},
		{"x35", `"0"`},
		{"x36", `"8"`},
		{"x37", `"0|0|0|0|0|0|0|0|0|1|0|0|0|0|0|0|0|0|1|0|0|0|0|0"`},
		{"x38", `"0|0|1|0|1|0|0|0|0|0|1|0|1|0|1|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0"`},
		{"x39", `0`},
		{"x42", `"3.4.4"`},
		{"x43", `"` + canvasHash + `"`},
		{"x44", `"1700000000000"`},
		{"x45", `"__SEC_CAV__1-1-1-1-1|__SEC_WSA__|"`},
		{"x46", `"false"`},
		{"x48", `""`},
		{"x49", `"{list:[],type:}"`},
		{"x50", `""`},
		{"x51", `""`},
		{"x52", `""`},
		{"x82", `"` + webglHash + `"`},
	}
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		parts = append(parts, `"`+f.k+`":`+f.v)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// Stable canvas/webgl hashes for the fixed fingerprint above.
const (
	canvasHash = "a1b2c3d4e5f60718293a4b5c6d7e8f90"
	webglHash  = "0f1e2d3c4b5a69788776655443322110"
)

func cookieString(cookies map[string]string) string {
	if len(cookies) == 0 {
		return ""
	}
	parts := make([]string, 0, len(cookies))
	for k, v := range cookies {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, "; ")
}

// GenerateA1 builds a synthetic a1 cookie value, the 52-character device token
// the browser would otherwise be issued. It is used only when the homepage
// bootstrap does not set a1 (a blocked IP, for instance). The shape is
// hex(ms) + 30 random [a-z0-9] + "50000" + crc32, truncated to 52 chars.
func GenerateA1(now time.Time, rnd func(n int) []byte) string {
	tsHex := strconv.FormatInt(now.UnixMilli(), 16)
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	raw := rnd(30)
	var sb strings.Builder
	for _, b := range raw {
		sb.WriteByte(charset[int(b)%len(charset)])
	}
	aPart := tsHex + sb.String() + "5" + "0" + "000"
	crc := crc32.ChecksumIEEE([]byte(aPart))
	full := aPart + strconv.FormatUint(uint64(crc), 10)
	if len(full) > 52 {
		full = full[:52]
	}
	return full
}

// WebID derives the webId cookie from a1: the md5 of a1 in hex.
func WebID(a1 string) string {
	sum := md5.Sum([]byte(a1))
	return hex.EncodeToString(sum[:])
}

func nowMillis() int64 { return time.Now().UnixMilli() }

func cryptoRand(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// Fall back to a time-seeded fill; randomness here only affects trace ids.
		t := time.Now().UnixNano()
		for i := range b {
			b[i] = byte(t >> (uint(i%8) * 8))
		}
	}
	return b
}
