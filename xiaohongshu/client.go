package xiaohongshu

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tamnd/xiaohongshu-cli/pkg/xhssign"
)

// dryRunOut is where DryRun prints the requests it would make.
var dryRunOut io.Writer = os.Stdout

// Client is a signed, paced, retrying Xiaohongshu web API client.
type Client struct {
	cfg    Config
	hc     *http.Client
	cache  *cache
	signer *xhssign.Signer
	nowFn  func() time.Time

	sessionOnce sync.Once

	mu   sync.Mutex
	next time.Time

	cookieMu sync.RWMutex
	cookies  map[string]string
}

// NewClient builds a client from cfg, filling defaults for zero fields.
func NewClient(cfg Config) *Client {
	if cfg.UserAgent == "" {
		cfg.UserAgent = DefaultUserAgent
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	tr := &http.Transport{}
	if cfg.Proxy != "" {
		if pu, err := url.Parse(cfg.Proxy); err == nil {
			tr.Proxy = http.ProxyURL(pu)
		}
	}
	c := &Client{
		cfg: cfg,
		hc: &http.Client{
			Timeout:       cfg.Timeout,
			Transport:     tr,
			CheckRedirect: stopAtRefusal,
		},
		signer:  xhssign.New(),
		nowFn:   time.Now,
		cookies: map[string]string{},
	}
	if !cfg.NoCache && cfg.CacheDir != "" {
		c.cache = newCache(cfg.CacheDir, cfg.CacheTTL)
	}
	c.applyCookies()
	return c
}

func (c *Client) now() time.Time { return c.nowFn() }

// SetNow overrides the clock (testing).
func (c *Client) SetNow(f func() time.Time) { c.nowFn = f }

// SetTransport replaces the HTTP transport (testing).
//
// This is how the tests drive the client end to end. An httptest server would
// be the obvious choice and cannot be used: it binds a socket, and every test
// binary in this repository has to pass with the network denied. A round
// tripper answers in process and needs no socket at all.
func (c *Client) SetTransport(rt http.RoundTripper) { c.hc.Transport = rt }

// applyCookies parses the configured cookie header into the per-request map.
func (c *Client) applyCookies() {
	if c.cfg.Cookie == "" {
		return
	}
	for part := range strings.SplitSeq(c.cfg.Cookie, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		c.setCookie(strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]))
	}
}

// captureCookies keeps every cookie either host hands out. The web host issues
// acw_tc, the Aliyun WAF token, with a thirty minute lifetime, and abRequestId
// alongside it. The previous version discarded both, so every request in a
// session arrived without the tokens a browser would be carrying by then.
func (c *Client) captureCookies(resp *http.Response) {
	for _, ck := range resp.Cookies() {
		if ck.Name == "" || ck.Value == "" {
			continue
		}
		c.setCookie(ck.Name, ck.Value)
	}
}

// refusalError turns a stopped redirect into the typed error the caller sees.
//
// A redirect to /login is ambiguous by construction: the site serves it both to
// a surface that wants a credential and to an address that has spent its
// budget, and nothing in the response tells the two apart. Resolving that costs
// one extra request against a page known to need no credential, which is what
// probeWeb does.
func (c *Client) refusalError(ctx context.Context, ref *refusal, endpoint string) error {
	st := classifyRefusal(ref)
	if st == StatusToken {
		return statusError(st, ref.errorCode(), ref.errorMessage(), endpoint)
	}
	return statusError(st, 0, "", endpoint)
}

func (c *Client) setCookie(name, value string) {
	c.cookieMu.Lock()
	c.cookies[name] = value
	c.cookieMu.Unlock()
}

func (c *Client) hasCookie(name string) bool {
	c.cookieMu.RLock()
	defer c.cookieMu.RUnlock()
	return c.cookies[name] != ""
}

func (c *Client) cookie(name string) string {
	c.cookieMu.RLock()
	defer c.cookieMu.RUnlock()
	return c.cookies[name]
}

func (c *Client) cookieHeader() string {
	c.cookieMu.RLock()
	defer c.cookieMu.RUnlock()
	if len(c.cookies) == 0 {
		return ""
	}
	parts := make([]string, 0, len(c.cookies))
	for k, v := range c.cookies {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, "; ")
}

func (c *Client) cookieSnapshot() map[string]string {
	c.cookieMu.RLock()
	defer c.cookieMu.RUnlock()
	m := make(map[string]string, len(c.cookies))
	maps.Copy(m, c.cookies)
	return m
}

// LoggedIn reports whether a web_session cookie is present, which is what the
// personalized surfaces need.
func (c *Client) LoggedIn() bool { return c.hasCookie("web_session") }

func (c *Client) throttle(ctx context.Context) error {
	if c.cfg.Rate <= 0 {
		return nil
	}
	c.mu.Lock()
	now := c.now()
	wait := c.next.Sub(now)
	if c.next.Before(now) {
		c.next = now.Add(c.cfg.Rate)
	} else {
		c.next = c.next.Add(c.cfg.Rate)
	}
	c.mu.Unlock()
	if wait <= 0 {
		return nil
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// envelope is the standard XHS response wrapper.
type envelope struct {
	Success bool            `json:"success"`
	Code    int             `json:"code"`
	Msg     string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}

var dryRunBody = []byte(`{"success":true,"code":0,"msg":"dry-run","data":null}`)

// GetJSON signs and performs a GET against the API host and decodes data.
func (c *Client) GetJSON(ctx context.Context, uri string, params map[string]string, out any) error {
	c.ensureSession(ctx)
	body, err := c.do(ctx, http.MethodGet, uri, params, "")
	if err != nil {
		return err
	}
	return decodeEnvelope(body, out)
}

// PostJSON signs and performs a POST with a JSON body and decodes data.
func (c *Client) PostJSON(ctx context.Context, uri string, payload any, out any) error {
	c.ensureSession(ctx)
	body := compactBody(payload)
	resp, err := c.do(ctx, http.MethodPost, uri, nil, body)
	if err != nil {
		return err
	}
	return decodeEnvelope(resp, out)
}

// Raw signs and performs the request, returning the untouched response body.
func (c *Client) Raw(ctx context.Context, method, uri string, params map[string]string, payload any) ([]byte, error) {
	c.ensureSession(ctx)
	body := ""
	if method == http.MethodPost {
		body = compactBody(payload)
	}
	return c.do(ctx, method, uri, params, body)
}

// do builds the signed request and runs it with retries.
func (c *Client) do(ctx context.Context, method, uri string, params map[string]string, body string) ([]byte, error) {
	full := Host + uri
	cacheKey := method + " " + xhssign.ContentString(method, uri, params, body)
	if c.cfg.DryRun {
		q := ""
		if len(params) > 0 {
			q = "?" + url.Values(toValues(params)).Encode()
		}
		_, _ = fmt.Fprintf(dryRunOut, "%s %s%s\n", method, full, q)
		return dryRunBody, nil
	}
	return c.runWithRetry(ctx, cacheKey, true, func(ctx context.Context) (result, error) {
		return c.attempt(ctx, method, full, uri, params, body)
	})
}

// runWithRetry runs attempt with the client's pacing and retry policy, reading
// and writing the on-disk cache under cacheKey when useCache is set. It backs
// off between tries and gives up after Retries attempts.
func (c *Client) runWithRetry(ctx context.Context, cacheKey string, useCache bool, attempt func(context.Context) (result, error)) ([]byte, error) {
	if c.cache != nil && useCache {
		if b, ok := c.cache.get(cacheKey); ok {
			return b, nil
		}
	}
	var last error
	tries := 0
	for try := 0; try <= c.cfg.Retries; try++ {
		if try > 0 {
			d := c.cfg.Rate * time.Duration(try*try+1)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(d):
			}
		}
		if err := c.throttle(ctx); err != nil {
			return nil, err
		}
		tries = try + 1
		res, err := attempt(ctx)
		if err != nil {
			// Retry only what a second request could plausibly answer
			// differently. A 404 is the routing table, a 302 to /login is the
			// caller's budget, and a 406 is the signature: none of the three is
			// different two seconds later, and retrying the second one is how a
			// rate-limited address stays rate-limited.
			if !StatusOf(err).Retryable() {
				return nil, err
			}
			last = err
			continue
		}
		// A refusal is never cached. It describes the caller's standing at a
		// moment rather than the object they asked for, and keeping one
		// poisons every later run from the same directory.
		if c.cache != nil && useCache && res.status.Cacheable() {
			c.cache.put(cacheKey, res.body)
		}
		return res.body, nil
	}
	return nil, &APIError{
		Message: last.Error(),
		Hint:    fmt.Sprintf("gave up after %d attempts", tries),
		Status:  StatusNetwork,
	}
}

// result is one completed attempt: the body and the state it was sorted into.
// The status travels with the body so the cache can refuse to keep a refusal
// without re-deriving what the response was.
type result struct {
	body   []byte
	status Status
}

func (c *Client) attempt(ctx context.Context, method, full, uri string, params map[string]string, body string) (result, error) {
	var reqBody io.Reader
	if method == http.MethodPost {
		reqBody = bytes.NewReader([]byte(body))
	}
	target := full
	if len(params) > 0 {
		target = full + "?" + url.Values(toValues(params)).Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reqBody)
	if err != nil {
		return result{}, err
	}
	headers := c.signer.Sign(xhssign.Request{
		Method:  method,
		URI:     uri,
		Params:  params,
		Body:    body,
		A1:      c.cookie("a1"),
		Cookies: c.cookieSnapshot(),
	})
	for k, v := range headers.Map() {
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Referer", Referer)
	req.Header.Set("Origin", Origin)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Encoding", "gzip")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	}
	if ck := c.cookieHeader(); ck != "" {
		req.Header.Set("Cookie", ck)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		var ref *refusal
		if errors.As(err, &ref) {
			return result{}, c.refusalError(ctx, ref, uri)
		}
		return result{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	c.captureCookies(resp)
	// A 429 and a 5xx are a server saying it is busy, which is the only thing
	// worth asking again about, so they leave as plain errors for the retry
	// loop rather than as classified refusals.
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return result{}, fmt.Errorf("HTTP %d from %s", resp.StatusCode, uri)
	}
	out, err := readBody(resp)
	if err != nil {
		return result{}, err
	}
	switch st := classifyAPI(resp.StatusCode, out); st {
	case StatusOK, StatusEmpty:
		return result{body: out, status: st}, nil
	case StatusError:
		return result{}, statusError(st, resp.StatusCode, oneLine(out), uri)
	default:
		var env envelope
		_ = json.Unmarshal(out, &env)
		return result{}, statusError(st, envCode(env, resp.StatusCode), env.Msg, uri)
	}
}

// envCode prefers the envelope's own code and falls back to the HTTP status,
// which is what a gateway refusal leaves behind when there is no envelope.
func envCode(env envelope, httpStatus int) int {
	if env.Code != 0 {
		return env.Code
	}
	return httpStatus
}

// oneLine trims a body down to something printable in an error message.
func oneLine(b []byte) string {
	s := strings.TrimSpace(string(b))
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}

func readBody(resp *http.Response) ([]byte, error) {
	var r io.Reader = resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer func() { _ = gz.Close() }()
		r = gz
	}
	return io.ReadAll(io.LimitReader(r, 64<<20))
}

func decodeEnvelope(body []byte, out any) error {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}
	if !env.Success && env.Code != 0 {
		return apiError(env.Code, env.Msg)
	}
	if out == nil || len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("decode data: %w", err)
	}
	return nil
}

func compactBody(payload any) string {
	if payload == nil {
		return "{}"
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func toValues(m map[string]string) map[string][]string {
	v := make(map[string][]string, len(m))
	for k, val := range m {
		v[k] = []string{val}
	}
	return v
}

// randBytes returns n bytes of randomness, used for synthetic a1 generation.
func randBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = byte(i * 7)
		}
	}
	return b
}
