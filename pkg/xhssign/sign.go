// Package xhssign builds the signed request headers that Xiaohongshu's web
// API (edith.xiaohongshu.com) requires: x-s, x-t, x-s-common, x-b3-traceid and
// x-xray-traceid.
//
// The scheme here is reverse-engineered from the public xiaohongshu.com web
// client. It is reimplemented in Go from the algorithm description; no upstream
// code is copied. Xiaohongshu rotates this scheme from time to time, so when
// the API starts rejecting requests the fix lives entirely in this package and
// its tests.
//
// The signing path is the modern XYW format. The older XYS format is rejected
// with HTTP 406 by the data endpoints (user_posted, otherinfo and friends), so
// XYW is the only path implemented.
package xhssign

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rc4"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Fixed constants of the web scheme. These are not secrets; they are the same
// values the browser ships.
const (
	customAlphabet = "ZmserbBoHQtNP+wOcza/LpngG8yJq42KWYj0DSfdikx3VT16IlUAFM97hECvuRX5"

	xywAESKey   = "7cc4adla5ay0701v"
	xywAESIV    = "4uzjr7mbsibcaldp"
	xywEnvFlags = "0|0|0|1|0|0|1|0|0|0|1|0|0|0|0|1|0|0|1"

	xywSignSvn     = "56"
	xywSignType    = "x2"
	xywSignVersion = "1"
	xywPrefix      = "XYW_"

	b1SecretKey = "xhswebmplfbt"

	// DefaultAppID is the web client's application identifier.
	DefaultAppID = "xhs-pc-web"

	// PublicUserAgent is the desktop Chrome identifier the fingerprint is built
	// against. The signed fingerprint and the request's User-Agent should agree.
	PublicUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36 Edg/142.0.0.0"
)

// customB64 is the standard base64 with the alphabet swapped for the XHS one.
var customB64 = base64.NewEncoding(customAlphabet).WithPadding('=')

// Headers is the signed header set for one request.
type Headers struct {
	XS          string
	XT          string
	XSCommon    string
	B3TraceID   string
	XrayTrace   string
	XYDirection string
}

// Map renders the headers as an http header map.
func (h Headers) Map() map[string]string {
	return map[string]string{
		"x-s":            h.XS,
		"x-t":            h.XT,
		"x-s-common":     h.XSCommon,
		"x-b3-traceid":   h.B3TraceID,
		"x-xray-traceid": h.XrayTrace,
		"x-mns":          "unload",
		"xy-direction":   h.XYDirection,
	}
}

// Request describes what to sign.
type Request struct {
	Method  string            // GET or POST
	URI     string            // path only, e.g. /api/sns/web/v1/feed
	Params  map[string]string // GET query params, in caller-supplied order via ParamOrder
	Body    string            // POST body, already compact JSON
	A1      string            // a1 cookie value
	Cookies map[string]string // full cookie set, for x-s-common
	AppID   string            // defaults to DefaultAppID
}

// Signer holds the injectable clock and randomness so tests are deterministic.
type Signer struct {
	// NowMillis returns the current time in milliseconds. Tests override it.
	NowMillis func() int64
	// Rand returns n bytes of randomness. Tests override it.
	Rand func(n int) []byte
}

// New returns a Signer using the real clock and crypto randomness.
func New() *Signer {
	return &Signer{NowMillis: nowMillis, Rand: cryptoRand}
}

// Sign produces the full header set for req at the given content string.
func (s *Signer) Sign(req Request) Headers {
	if req.AppID == "" {
		req.AppID = DefaultAppID
	}
	ts := s.NowMillis()
	content := ContentString(req.Method, req.URI, req.Params, req.Body)
	xs := buildXS(content, req.A1, req.AppID, ts)
	return Headers{
		XS:          xs,
		XT:          strconv.FormatInt(ts, 10),
		XSCommon:    buildXSCommon(req.A1, req.Cookies),
		B3TraceID:   s.b3TraceID(),
		XrayTrace:   s.xrayTraceID(ts),
		XYDirection: strconv.Itoa(shardingKey(req.Cookies["webId"])),
	}
}

// ContentString builds the string the x-s payload hashes over. For GET it is
// the path plus an ordered query; for POST it is the path concatenated with the
// compact JSON body.
func ContentString(method, uri string, params map[string]string, body string) string {
	if strings.EqualFold(method, "POST") {
		return uri + body
	}
	if len(params) == 0 {
		return uri
	}
	keys := orderedKeys(params)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		// Commas are left unescaped, matching the browser's encoder.
		v := strings.ReplaceAll(url.QueryEscape(params[k]), "%2C", ",")
		parts = append(parts, k+"="+v)
	}
	return uri + "?" + strings.Join(parts, "&")
}

// orderedKeys returns map keys sorted, so signing is deterministic. The browser
// uses object insertion order; callers that need a specific order should build
// the body/content themselves. Sorted order is stable and works for the GET
// endpoints used here, which carry few params.
func orderedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// buildXS produces the XYW_ x-s header value.
func buildXS(content, a1, appID string, tsMillis int64) string {
	x1 := md5Hex("url=" + content)
	message := fmt.Sprintf("x1=%s;x2=%s;x3=%s;x4=%d;", x1, xywEnvFlags, a1, tsMillis)
	inner := []byte(base64.StdEncoding.EncodeToString([]byte(message)))
	payloadHex := hex.EncodeToString(aesCBCEncrypt(pkcs7Pad(inner), []byte(xywAESKey), []byte(xywAESIV)))

	doc := map[string]string{
		"signSvn":     xywSignSvn,
		"signType":    xywSignType,
		"appId":       appID,
		"signVersion": xywSignVersion,
		"payload":     payloadHex,
	}
	j := compactJSON(doc, []string{"signSvn", "signType", "appId", "signVersion", "payload"})
	return xywPrefix + base64.StdEncoding.EncodeToString([]byte(j))
}

// buildXSCommon produces the x-s-common header value.
func buildXSCommon(a1 string, cookies map[string]string) string {
	b1 := generateB1(cookies)
	x9 := crc32JS(b1)
	// Field order matters: the template is a JS object with a fixed key order.
	parts := []string{
		`"s0":5`,
		`"s1":""`,
		`"x0":"1"`,
		`"x1":"4.3.5"`,
		`"x2":"Windows"`,
		`"x3":"xhs-pc-web"`,
		`"x4":"4.86.0"`,
		`"x5":` + jsonString(a1),
		`"x6":""`,
		`"x7":""`,
		`"x8":` + jsonString(b1),
		`"x9":` + strconv.FormatInt(int64(x9), 10),
		`"x10":0`,
		`"x11":"normal"`,
	}
	doc := "{" + strings.Join(parts, ",") + "}"
	return customB64.EncodeToString([]byte(doc))
}

// --- crypto primitives ---

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func pkcs7Pad(b []byte) []byte {
	pad := aes.BlockSize - len(b)%aes.BlockSize
	out := make([]byte, len(b)+pad)
	copy(out, b)
	for i := len(b); i < len(out); i++ {
		out[i] = byte(pad)
	}
	return out
}

func aesCBCEncrypt(plain, key, iv []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		// key is a fixed 16-byte constant, so this never fails.
		panic(err)
	}
	out := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, plain)
	return out
}

// crc32JS reproduces the site's JS CRC32 variant: it returns the signed 32-bit
// value of the IEEE checksum XORed with the polynomial 0xEDB88320.
func crc32JS(s string) int32 {
	c := crc32.ChecksumIEEE([]byte(s))
	return int32(c ^ 0xEDB88320)
}

// generateB1 builds the b1 browser-fingerprint token: RC4-encrypt the
// fingerprint JSON, percent-encode it, expand the escapes to bytes, then encode
// with the custom base64 alphabet.
func generateB1(cookies map[string]string) string {
	fp := fingerprintB1(cookies)
	cipher, _ := rc4.NewCipher([]byte(b1SecretKey))
	enc := make([]byte, len(fp))
	cipher.XORKeyStream(enc, []byte(fp))

	escaped := percentEscape(string(enc))
	var b []byte
	segs := strings.Split(escaped, "%")
	for _, c := range segs[1:] {
		if len(c) < 2 {
			continue
		}
		v, err := strconv.ParseUint(c[:2], 16, 16)
		if err != nil {
			continue
		}
		b = append(b, byte(v))
		for _, r := range c[2:] {
			b = append(b, byte(r))
		}
	}
	return customB64.EncodeToString(b)
}

// percentEscape mirrors Python's urllib.parse.quote with safe="!*'()~_-": it
// keeps the unreserved set [A-Za-z0-9_.-] plus those safe characters and
// percent-encodes every other byte uppercase.
func percentEscape(s string) string {
	const safe = "!*'()~_-"
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '.' || strings.IndexByte(safe, c) >= 0 {
			sb.WriteByte(c)
			continue
		}
		fmt.Fprintf(&sb, "%%%02X", c)
	}
	return sb.String()
}

// --- trace ids and sharding ---

func (s *Signer) b3TraceID() string {
	return hexFrom(s.Rand(8))
}

func (s *Signer) xrayTraceID(tsMillis int64) string {
	seq := int64(beUint(s.Rand(4)) & 0x7FFFFF)
	part1 := fmt.Sprintf("%016x", (tsMillis<<23)|seq)
	if len(part1) > 16 {
		part1 = part1[len(part1)-16:]
	}
	return part1 + hexFrom(s.Rand(8))
}

// shardingKey maps an id to the xy-direction shard. Without a user id the
// browser uses a random value in a small range; a stable hash is fine here.
func shardingKey(id string) int {
	if id == "" {
		return 0
	}
	return int(crc32.ChecksumIEEE([]byte(id)) % 1000)
}

// --- json helpers ---

func compactJSON(m map[string]string, order []string) string {
	parts := make([]string, 0, len(order))
	for _, k := range order {
		parts = append(parts, jsonString(k)+":"+jsonString(m[k]))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func hexFrom(b []byte) string { return hex.EncodeToString(b) }

func beUint(b []byte) uint32 {
	var v uint32
	for _, x := range b {
		v = v<<8 | uint32(x)
	}
	return v
}
