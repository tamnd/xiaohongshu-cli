package xiaohongshu

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"
)

// cache is a simple on-disk blob cache keyed by a hash of the request signature.
type cache struct {
	dir string
	ttl time.Duration
}

func newCache(dir string, ttl time.Duration) *cache {
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &cache{dir: dir, ttl: ttl}
}

func (c *cache) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	h := hex.EncodeToString(sum[:])
	return filepath.Join(c.dir, h[:2], h+".json")
}

func (c *cache) get(key string) ([]byte, bool) {
	p := c.path(key)
	fi, err := os.Stat(p)
	if err != nil {
		return nil, false
	}
	if time.Since(fi.ModTime()) > c.ttl {
		return nil, false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	return b, true
}

func (c *cache) put(key string, data []byte) {
	p := c.path(key)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, p)
}

// ClearCache removes every cached entry and returns the count removed.
func ClearCache(dir string) (int, error) {
	n := 0
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	for _, e := range entries {
		sub := filepath.Join(dir, e.Name())
		if e.IsDir() {
			files, _ := os.ReadDir(sub)
			n += len(files)
		}
		_ = os.RemoveAll(sub)
	}
	return n, nil
}

// CacheStats returns the number of cached files and total bytes.
func CacheStats(dir string) (files int, bytes int64) {
	_ = filepath.Walk(dir, func(_ string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		files++
		bytes += fi.Size()
		return nil
	})
	return
}
