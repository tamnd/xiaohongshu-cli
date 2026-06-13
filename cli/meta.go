package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tamnd/xiaohongshu-cli/pkg/xhsurl"
	"github.com/tamnd/xiaohongshu-cli/xiaohongshu"
)

// ---- version ----

func newVersionCmd(a *App) *cobra.Command {
	var short bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version, commit, and build date",
		RunE: func(_ *cobra.Command, _ []string) error {
			if short {
				fmt.Println(Version)
				return nil
			}
			info := map[string]string{"version": Version, "commit": Commit, "date": Date}
			if a.resolveFormat() == FormatTable {
				fmt.Printf("xhs %s (%s) built %s\n", Version, Commit, Date)
				return nil
			}
			b, _ := json.MarshalIndent(info, "", "  ")
			fmt.Println(string(b))
			return nil
		},
	}
	cmd.Flags().BoolVar(&short, "short", false, "print just the version string")
	return cmd
}

// ---- cache ----

func newCacheCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Inspect or clear the on-disk response cache",
	}
	dir := xiaohongshu.DefaultConfig().CacheDir

	stat := &cobra.Command{
		Use:   "stat",
		Short: "Show cache location, file count, and size",
		RunE: func(_ *cobra.Command, _ []string) error {
			files, bytes := xiaohongshu.CacheStats(dir)
			return emitOne(a, map[string]any{
				"dir": dir, "files": files, "bytes": bytes, "size": humanBytes(bytes),
			})
		},
	}
	clear := &cobra.Command{
		Use:   "clear",
		Short: "Delete every cached response",
		RunE: func(_ *cobra.Command, _ []string) error {
			n, err := xiaohongshu.ClearCache(dir)
			if err != nil {
				return err
			}
			a.progress("removed %d cached files from %s", n, dir)
			return nil
		},
	}
	path := &cobra.Command{
		Use:   "path",
		Short: "Print the cache directory",
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Println(dir)
			return nil
		},
	}
	cmd.AddCommand(stat, clear, path)
	return cmd
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// ---- config ----

func newConfigCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show resolved configuration and important paths",
	}
	show := &cobra.Command{
		Use:   "show",
		Short: "Print effective settings (secrets redacted)",
		RunE: func(_ *cobra.Command, _ []string) error {
			cookie := a.resolveCookie()
			return emitOne(a, map[string]any{
				"cache_dir":    xiaohongshu.DefaultConfig().CacheDir,
				"config_dir":   xiaohongshu.ConfigDir(),
				"session_path": xiaohongshu.SessionPath(),
				"user_agent":   firstNonEmpty(a.userAgent, os.Getenv("XHS_USER_AGENT"), xiaohongshu.DefaultUserAgent),
				"cookie_set":   cookie != "",
				"cookie":       redactCookie(cookie),
				"rate":         a.rate.String(),
				"retries":      a.retries,
				"timeout":      a.timeout.String(),
				"cache_ttl":    a.cacheTTL.String(),
				"proxy":        firstNonEmpty(a.proxy, os.Getenv("XHS_PROXY")),
			})
		},
	}
	paths := &cobra.Command{
		Use:   "path",
		Short: "Print config, cache, and session paths",
		RunE: func(_ *cobra.Command, _ []string) error {
			return emitOne(a, map[string]any{
				"config_dir":   xiaohongshu.ConfigDir(),
				"cache_dir":    xiaohongshu.DefaultConfig().CacheDir,
				"session_path": xiaohongshu.SessionPath(),
			})
		},
	}
	cmd.AddCommand(show, paths)
	return cmd
}

func redactCookie(c string) string {
	if c == "" {
		return ""
	}
	parts := strings.Split(c, ";")
	for i, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) == 2 {
			v := kv[1]
			if len(v) > 6 {
				v = v[:3] + "…" + v[len(v)-3:]
			} else {
				v = "…"
			}
			parts[i] = kv[0] + "=" + v
		}
	}
	return strings.Join(parts, "; ")
}

// ---- session ----

func newSessionCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Inspect or reset the persisted anonymous web session",
	}
	show := &cobra.Command{
		Use:   "show",
		Short: "Print the persisted session path and whether one exists",
		RunE: func(_ *cobra.Command, _ []string) error {
			p := xiaohongshu.SessionPath()
			_, err := os.Stat(p)
			return emitOne(a, map[string]any{"path": p, "exists": err == nil})
		},
	}
	forget := &cobra.Command{
		Use:   "forget",
		Short: "Delete the persisted session so the next call bootstraps a fresh one",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := xiaohongshu.ForgetSession(); err != nil {
				return err
			}
			a.progress("forgot session at %s", xiaohongshu.SessionPath())
			return nil
		},
	}
	cmd.AddCommand(show, forget)
	return cmd
}

// ---- crawl ----

// crawlNode is one entry in the breadth-first frontier: a note to fetch and the
// hop distance from a seed.
type crawlNode struct {
	id    string
	token string
	depth int
}

// newCrawlCmd is the scraping engine. It seeds a frontier from the explore feed
// and from any note ids passed in, then walks outward breadth-first: each note
// can reach its author, the author's other notes, and its related notes. Every
// record kind streams to its own JSONL file as it is found, so a long crawl
// leaves usable output even if it is interrupted. Notes and users are
// de-duplicated, and depth and max bound the walk.
func newCrawlCmd(a *App) *cobra.Command {
	var outDir, token, category string
	var explore, withComments, withRelated, withAuthorNotes, withUser, deep bool
	var depth, maxNotes, seedLimit int
	cmd := &cobra.Command{
		Use:   "crawl [id|url]...",
		Short: "Crawl connected records into JSONL files, seeding from explore or note ids",
		Args:  cobra.ArbitraryArgs,
		Example: "  xhs crawl --explore --depth 2 --max 500 --out ./data\n" +
			"  xhs crawl --category food --author-notes --out ./food\n" +
			"  xhs crawl <note-id> --token <t> --related --comments --out ./data\n" +
			"  xhs search coffee -o url | xhs crawl - --out ./data",
		RunE: func(_ *cobra.Command, args []string) error {
			if outDir == "" {
				outDir = "."
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return err
			}
			c := a.Client()
			seeds := readArgsOrStdin(args)
			if !explore && category == "" && len(seeds) == 0 {
				return fmt.Errorf("nothing to crawl: pass note ids or use --explore / --category")
			}

			nw := newJSONLWriter(filepath.Join(outDir, "notes.jsonl"))
			defer func() { _ = nw.Close() }()
			uw := newJSONLWriter(filepath.Join(outDir, "users.jsonl"))
			defer func() { _ = uw.Close() }()
			cw := newJSONLWriter(filepath.Join(outDir, "comments.jsonl"))
			defer func() { _ = cw.Close() }()

			seenNote := map[string]bool{}
			seenUser := map[string]bool{}
			var frontier []crawlNode
			enqueue := func(id, tok string, d int) {
				if id == "" || seenNote[id] {
					return
				}
				seenNote[id] = true
				frontier = append(frontier, crawlNode{id: id, token: tok, depth: d})
			}

			// Seed from the explore feed. Each fetch is reshuffled, so this pulls
			// a wide, fresh starting set without a cookie.
			if explore || category != "" {
				cat := firstNonEmpty(category, "recommend")
				seeded := 0
				for fi, err := range c.Feed(a.ctx(), cat, seedLimit) {
					if err != nil {
						a.progress("seed explore/%s: %v", cat, err)
						break
					}
					enqueue(fi.NoteID, fi.XsecToken, 0)
					seeded++
				}
				a.progress("seeded %d notes from explore/%s", seeded, cat)
			}
			for _, seed := range seeds {
				ref := xhsurl.Parse(seed)
				enqueue(ref.NoteID, firstNonEmpty(token, ref.XsecToken), 0)
			}

			processed := 0
			for len(frontier) > 0 {
				if maxNotes > 0 && processed >= maxNotes {
					a.progress("reached --max %d, stopping", maxNotes)
					break
				}
				cur := frontier[0]
				frontier = frontier[1:]

				n, err := c.Note(a.ctx(), cur.id, cur.token)
				if err != nil {
					a.progress("skip note %s: %v", cur.id, err)
					continue
				}
				nw.Write(n)
				processed++
				a.progress("note %s %q (depth %d, %d done, %d queued)", n.NoteID, n.Title, cur.depth, processed, len(frontier))

				if withUser && n.UserID != "" && !seenUser[n.UserID] {
					seenUser[n.UserID] = true
					if u, err := c.User(a.ctx(), n.UserID); err == nil {
						uw.Write(u)
					} else {
						a.progress("skip user %s: %v", n.UserID, err)
					}
					if withAuthorNotes && cur.depth < depth {
						for fi, err := range c.UserNotes(a.ctx(), n.UserID, a.limit) {
							if err != nil {
								break
							}
							enqueue(fi.NoteID, fi.XsecToken, cur.depth+1)
						}
					}
				}
				if withRelated && cur.depth < depth {
					if rel, err := c.Related(a.ctx(), n.NoteID, cur.token, a.limit); err == nil {
						for _, r := range rel {
							enqueue(r.NoteID, r.XsecToken, cur.depth+1)
						}
					}
				}
				if withComments {
					cnt := 0
					for cm, err := range c.Comments(a.ctx(), n.NoteID, cur.token, deep, a.limit) {
						if err != nil {
							break
						}
						cw.Write(cm)
						cnt++
					}
					if cnt > 0 {
						a.progress("  %d comments", cnt)
					}
				}
			}
			a.progress("crawl done: %d notes, %d users into %s", processed, len(seenUser), outDir)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&outDir, "out", ".", "output directory for JSONL files")
	f.StringVar(&token, "token", "", "xsec_token shared by the seed notes")
	f.BoolVar(&explore, "explore", false, "seed the frontier from the explore feed")
	f.StringVar(&category, "category", "", "seed from a named explore category (implies --explore)")
	f.IntVar(&seedLimit, "seed-limit", 30, "how many explore notes to seed with")
	f.IntVar(&depth, "depth", 1, "how many hops to follow related and author notes")
	f.IntVar(&maxNotes, "max", 0, "stop after this many notes (0 means no cap)")
	f.BoolVar(&withComments, "comments", false, "also crawl comments")
	f.BoolVar(&deep, "deep", false, "include comment replies when crawling comments")
	f.BoolVar(&withRelated, "related", true, "follow related notes into the frontier")
	f.BoolVar(&withAuthorNotes, "author-notes", false, "follow each author's other notes into the frontier")
	f.BoolVar(&withUser, "user", true, "also crawl the note author profile")
	return cmd
}

type jsonlWriter struct {
	f   *os.File
	enc *json.Encoder
	err error
}

func newJSONLWriter(path string) *jsonlWriter {
	f, err := os.Create(path)
	if err != nil {
		return &jsonlWriter{err: err}
	}
	return &jsonlWriter{f: f, enc: json.NewEncoder(f)}
}

func (w *jsonlWriter) Write(v any) {
	if w.err != nil || w.enc == nil {
		return
	}
	w.err = w.enc.Encode(v)
}

func (w *jsonlWriter) Close() error {
	if w.f != nil {
		return w.f.Close()
	}
	return w.err
}
