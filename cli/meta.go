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

// newCrawlCmd walks outward from seed notes or users, writing connected records
// as JSONL (one file per record kind) into an output directory.
func newCrawlCmd(a *App) *cobra.Command {
	var outDir, token string
	var withComments, withRelated, withUser, deep bool
	cmd := &cobra.Command{
		Use:     "crawl <id|url>...",
		Short:   "Crawl connected records from seed notes into JSONL files",
		Args:    cobra.MinimumNArgs(1),
		Example: "  xhs crawl <note-id> --token <t> --out ./data --comments\n  xhs search coffee -o url | xhs crawl - --out ./data",
		RunE: func(_ *cobra.Command, args []string) error {
			if outDir == "" {
				outDir = "."
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return err
			}
			c := a.Client()
			seeds := readArgsOrStdin(args)

			nw := newJSONLWriter(filepath.Join(outDir, "notes.jsonl"))
			defer func() { _ = nw.Close() }()
			uw := newJSONLWriter(filepath.Join(outDir, "users.jsonl"))
			defer func() { _ = uw.Close() }()
			cw := newJSONLWriter(filepath.Join(outDir, "comments.jsonl"))
			defer func() { _ = cw.Close() }()

			seenUser := map[string]bool{}
			for _, seed := range seeds {
				ref := xhsurl.Parse(seed)
				tok := firstNonEmpty(token, ref.XsecToken)
				n, err := c.Note(a.ctx(), ref.NoteID, tok)
				if err != nil {
					a.progress("skip %s: %v", ref.NoteID, err)
					continue
				}
				nw.Write(n)
				a.progress("note %s %q", n.NoteID, n.Title)

				if withUser && n.UserID != "" && !seenUser[n.UserID] {
					seenUser[n.UserID] = true
					if u, err := c.User(a.ctx(), n.UserID); err == nil {
						uw.Write(u)
					}
				}
				if withRelated {
					if rel, err := c.Related(a.ctx(), n.NoteID, tok, a.limit); err == nil {
						for _, r := range rel {
							nw.Write(r)
						}
					}
				}
				if withComments {
					cnt := 0
					for cm, err := range c.Comments(a.ctx(), n.NoteID, tok, deep, a.limit) {
						if err != nil {
							break
						}
						cw.Write(cm)
						cnt++
					}
					a.progress("  %d comments", cnt)
				}
			}
			a.progress("wrote records to %s", outDir)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&outDir, "out", ".", "output directory for JSONL files")
	f.StringVar(&token, "token", "", "xsec_token shared by the seed notes")
	f.BoolVar(&withComments, "comments", false, "also crawl comments")
	f.BoolVar(&deep, "deep", false, "include comment replies when crawling comments")
	f.BoolVar(&withRelated, "related", true, "also crawl related notes")
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
