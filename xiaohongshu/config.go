// Package xiaohongshu is the library behind the xhs command line: the signed
// HTTP client, the anonymous web session, and the typed records for notes,
// users, comments, search, feed, and tags.
//
// Xiaohongshu's web API is guarded. It blocks datacenter IP ranges within
// minutes and rate-limits each IP hard. From a residential IP the public
// surfaces are reachable; from a server or CI most of them are walled and
// return risk-control codes. The deeper, personalized surfaces always need a
// logged-in cookie. The client paces itself, retries the transient failures,
// and sends an honest User-Agent so a careful session stays usable.
package xiaohongshu

import (
	"os"
	"path/filepath"
	"time"
)

// Host is the JSON API host. Bootstrap happens on www.xiaohongshu.com.
const (
	Host    = "https://edith.xiaohongshu.com"
	WebHost = "https://www.xiaohongshu.com"
	Referer = "https://www.xiaohongshu.com/"
	Origin  = "https://www.xiaohongshu.com"
)

// DefaultUserAgent is a realistic desktop Chrome UA. XHS rejects unusual or
// empty agents and signs its fingerprint against a UA like this one.
const DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36 Edg/142.0.0.0"

// Config holds everything the client needs. The zero value is usable after
// passing through DefaultConfig.
type Config struct {
	Cookie    string
	UserAgent string
	Proxy     string

	Rate    time.Duration
	Retries int
	Timeout time.Duration

	CacheDir string
	CacheTTL time.Duration
	NoCache  bool

	// DryRun prints each request's method and URL instead of hitting the network.
	DryRun bool
}

// DefaultConfig returns polite defaults. The rate is deliberately slow because
// XHS throttles bursts of anonymous traffic.
func DefaultConfig() Config {
	return Config{
		UserAgent: DefaultUserAgent,
		Rate:      600 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
		CacheDir:  cacheDir(),
		CacheTTL:  time.Hour,
	}
}

func cacheDir() string {
	if d := os.Getenv("XHS_CACHE_DIR"); d != "" {
		return d
	}
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "xhs")
	}
	return filepath.Join(os.TempDir(), "xhs-cache")
}

// ConfigDir is where the persisted anon session lives.
func ConfigDir() string {
	if d := os.Getenv("XHS_CONFIG_DIR"); d != "" {
		return d
	}
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "xhs")
	}
	return filepath.Join(os.TempDir(), "xhs-config")
}
