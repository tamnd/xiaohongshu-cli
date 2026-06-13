package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tamnd/xiaohongshu-cli/pkg/xhshtml"
	"github.com/tamnd/xiaohongshu-cli/pkg/xhsurl"
)

// dryRunHTML is what a web fetch returns under --dry-run: an empty state so the
// parsers run cleanly and yield no records.
const dryRunHTML = `<script>window.__INITIAL_STATE__={}</script>`

// getState fetches a server-rendered page and returns its __INITIAL_STATE__
// object. The anonymous surfaces (note, user, feed, related) read this instead
// of the JSON API, which refuses anonymous callers with a login error. When
// fresh is set the on-disk cache is bypassed so a discovery loop sees a newly
// reshuffled page on every call.
func (c *Client) getState(ctx context.Context, path string, params map[string]string, fresh bool) (json.RawMessage, error) {
	c.ensureSession(ctx)
	html, err := c.doWeb(ctx, path, params, fresh)
	if err != nil {
		return nil, err
	}
	st, err := xhshtml.State(html)
	if err != nil {
		return nil, apiError(-101, "no anonymous data on this page (the token expired, or this surface needs a logged-in cookie)")
	}
	return st, nil
}

// doWeb fetches a page from the web host with browser-like headers and no
// signing. It reuses the client's pacing, retries, and cache.
func (c *Client) doWeb(ctx context.Context, path string, params map[string]string, fresh bool) ([]byte, error) {
	full := WebHost + path
	q := ""
	if len(params) > 0 {
		q = "?" + url.Values(toValues(params)).Encode()
	}
	if c.cfg.DryRun {
		_, _ = fmt.Fprintf(dryRunOut, "GET %s%s\n", full, q)
		return []byte(dryRunHTML), nil
	}
	cacheKey := "WEB " + full + q
	return c.runWithRetry(ctx, cacheKey, !fresh, func(ctx context.Context) ([]byte, error) {
		return c.webAttempt(ctx, full+q, path)
	})
}

func (c *Client) webAttempt(ctx context.Context, target, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en,zh-CN;q=0.9,zh;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip")
	// No Referer on purpose. XHS renders the server-side state for a direct
	// visit but treats an in-site referer as an SPA navigation and redirects it
	// to the login page, which is what walls the profile surface.
	if ck := c.cookieHeader(); ck != "" {
		req.Header.Set("Cookie", ck)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		// A redirect to the login page surfaces as a CheckRedirect stop below;
		// treat it as a rate or login wall rather than a transport error.
		if strings.Contains(err.Error(), errLoginWall) {
			return nil, apiError(-101, "redirected to login (rate-limited or this surface needs a logged-in cookie)")
		}
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	// XHS serves some surfaces with a 404 status but a fully rendered body, so
	// the presence of a state object matters more than the code. Only retry on
	// throttling and server faults.
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, path)
	}
	return readBody(resp)
}

// ---- shared server-rendered shapes (camelCase, unlike the snake_case API) ----

type ssrUserBrief struct {
	UserID   string `json:"userId"`
	Nickname string `json:"nickname"`
	NickName string `json:"nickName"`
	Avatar   string `json:"avatar"`
}

func (u ssrUserBrief) name() string {
	if u.Nickname != "" {
		return u.Nickname
	}
	return u.NickName
}

// ssrFeedItem is one card in the explore feed, a user's note grid, or a related
// list. The note id sits at the item level on the explore feed and inside the
// card on a profile grid, so both are read.
type ssrFeedItem struct {
	ID        string `json:"id"`
	XsecToken string `json:"xsecToken"`
	NoteCard  struct {
		NoteID       string       `json:"noteId"`
		Type         string       `json:"type"`
		DisplayTitle string       `json:"displayTitle"`
		User         ssrUserBrief `json:"user"`
		Cover        struct {
			URLDefault string `json:"urlDefault"`
			URL        string `json:"url"`
		} `json:"cover"`
		InteractInfo struct {
			LikedCount string `json:"likedCount"`
		} `json:"interactInfo"`
	} `json:"noteCard"`
}

func (c *Client) feedItem(it ssrFeedItem) FeedItem {
	id := firstNonEmpty(it.ID, it.NoteCard.NoteID)
	return FeedItem{
		NoteID:     id,
		Type:       it.NoteCard.Type,
		Title:      it.NoteCard.DisplayTitle,
		UserID:     it.NoteCard.User.UserID,
		Nickname:   it.NoteCard.User.name(),
		Cover:      firstNonEmpty(it.NoteCard.Cover.URLDefault, it.NoteCard.Cover.URL),
		LikedCount: humanCount(it.NoteCard.InteractInfo.LikedCount),
		XsecToken:  it.XsecToken,
		URL:        xhsurl.NoteURL(id, it.XsecToken),
		FetchedAt:  c.now().UTC().Format(time.RFC3339),
	}
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// humanCount parses the human-formatted counts the web pages carry, such as
// "1.9万" or "10万+", into an approximate integer. Plain numbers pass through.
func humanCount(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	s = strings.TrimSuffix(s, "+")
	mult := 1.0
	switch {
	case strings.HasSuffix(s, "万"):
		mult, s = 1e4, strings.TrimSuffix(s, "万")
	case strings.HasSuffix(s, "亿"):
		mult, s = 1e8, strings.TrimSuffix(s, "亿")
	case strings.HasSuffix(s, "w"), strings.HasSuffix(s, "W"):
		mult, s = 1e4, s[:len(s)-len("w")]
	case strings.HasSuffix(s, "k"), strings.HasSuffix(s, "K"):
		mult, s = 1e3, s[:len(s)-len("k")]
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return int64(f * mult)
}
