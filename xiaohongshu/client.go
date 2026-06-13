package xiaohongshu

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/json"
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
		cfg:     cfg,
		hc:      &http.Client{Timeout: cfg.Timeout, Transport: tr},
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
	if c.cache != nil && !c.cfg.DryRun {
		if b, ok := c.cache.get(cacheKey); ok {
			return b, nil
		}
	}
	if c.cfg.DryRun {
		q := ""
		if len(params) > 0 {
			q = "?" + url.Values(toValues(params)).Encode()
		}
		_, _ = fmt.Fprintf(dryRunOut, "%s %s%s\n", method, full, q)
		return dryRunBody, nil
	}

	var last error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			d := c.cfg.Rate * time.Duration(attempt*attempt+1)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(d):
			}
		}
		if err := c.throttle(ctx); err != nil {
			return nil, err
		}
		b, err := c.attempt(ctx, method, full, uri, params, body)
		if err != nil {
			last = err
			continue
		}
		if c.cache != nil {
			c.cache.put(cacheKey, b)
		}
		return b, nil
	}
	return nil, &APIError{Message: last.Error(), Hint: "request failed after retries", Kind: ErrNetwork}
}

func (c *Client) attempt(ctx context.Context, method, full, uri string, params map[string]string, body string) ([]byte, error) {
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
		return nil, err
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
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, uri)
	}
	out, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 406 || resp.StatusCode == 461 {
		return nil, apiError(resp.StatusCode, "anti-bot rejection")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, uri)
	}
	return out, nil
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
