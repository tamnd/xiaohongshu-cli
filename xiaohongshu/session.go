package xiaohongshu

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/tamnd/xiaohongshu-cli/pkg/xhssign"
)

// sessionTTL is how long a persisted anon session is reused before refreshing.
const sessionTTL = 12 * time.Hour

// anonSession is the persisted anonymous web session. The a1 and webId cookies
// are what every signed request needs; web_session arrives only after a real
// browser login and so is supplied through --cookie, never minted here.
type anonSession struct {
	A1        string    `json:"a1"`
	WebID     string    `json:"web_id"`
	CreatedAt time.Time `json:"created_at"`
}

// SessionPath is the on-disk location of the persisted anon session.
func SessionPath() string {
	return filepath.Join(ConfigDir(), "session.json")
}

func loadSession() (anonSession, bool) {
	b, err := os.ReadFile(SessionPath())
	if err != nil {
		return anonSession{}, false
	}
	var s anonSession
	if json.Unmarshal(b, &s) != nil || s.A1 == "" {
		return anonSession{}, false
	}
	return s, true
}

func saveSession(s anonSession) {
	p := SessionPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(p, b, 0o600)
}

// ForgetSession removes the persisted anon session.
func ForgetSession() error {
	err := os.Remove(SessionPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ensureSession makes sure the client has an a1 cookie. If the caller already
// supplied a1 via --cookie it is used as-is. Otherwise a persisted session is
// reused, or a fresh one is bootstrapped by visiting the homepage once and
// reading the a1 cookie the server sets. If the homepage does not set a1 (a
// blocked IP), a synthetic a1 is generated so requests can still be signed.
func (c *Client) ensureSession(ctx context.Context) {
	c.sessionOnce.Do(func() {
		if c.hasCookie("a1") {
			if !c.hasCookie("webId") {
				c.setCookie("webId", xhssign.WebID(c.cookie("a1")))
			}
			return
		}
		if s, ok := loadSession(); ok && time.Since(s.CreatedAt) < sessionTTL {
			c.setCookie("a1", s.A1)
			c.setCookie("webId", s.WebID)
			return
		}
		a1, webID := c.bootstrap(ctx)
		if a1 == "" {
			a1 = xhssign.GenerateA1(time.Now(), randBytes)
			webID = xhssign.WebID(a1)
		}
		c.setCookie("a1", a1)
		c.setCookie("webId", webID)
		saveSession(anonSession{A1: a1, WebID: webID, CreatedAt: time.Now()})
	})
}

// bootstrap visits the homepage and returns the a1 and webId cookies the server
// set, if any. It never fails the request path: a blank result just triggers
// the synthetic fallback.
func (c *Client) bootstrap(ctx context.Context) (a1, webID string) {
	if c.cfg.DryRun {
		return "", ""
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, WebHost+"/explore", nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := c.hc.Do(req)
	if err != nil {
		return "", ""
	}
	defer func() { _ = resp.Body.Close() }()
	for _, ck := range resp.Cookies() {
		switch ck.Name {
		case "a1":
			a1 = ck.Value
		case "webId":
			webID = ck.Value
		}
	}
	if a1 != "" && webID == "" {
		webID = xhssign.WebID(a1)
	}
	return a1, webID
}
