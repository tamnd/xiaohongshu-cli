// Package cli builds the xhs command tree on top of the xiaohongshu library.
package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/tamnd/xiaohongshu-cli/xiaohongshu"
)

// Build metadata, set via -ldflags at release time.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// App holds the global flags and the lazily built client shared by every command.
type App struct {
	output     string
	fields     string
	noHeader   bool
	template   string
	limit      int
	cookie     string
	cookieFile string
	rate       time.Duration
	retries    int
	timeout    time.Duration
	cache      bool
	noCache    bool
	cacheTTL   time.Duration
	quiet      bool
	verbose    int
	proxy      string
	userAgent  string
	raw        bool
	dryRun     bool

	client  *xiaohongshu.Client
	rootCtx context.Context
}

// Root builds the root command and its subtree.
func Root() *cobra.Command {
	app := &App{}
	root := &cobra.Command{
		Use:   "xhs",
		Short: "A command line for Xiaohongshu.",
		Long: "xhs reads public data from xiaohongshu.com and prints clean, pipeable\n" +
			"records: notes, users, comments, search results, the homefeed, and topics.\n" +
			"It signs its own requests and bootstraps an anonymous web session. The\n" +
			"deeper surfaces need a logged-in cookie and a residential IP; see the README.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			app.rootCtx = cmd.Context()
		},
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&app.output, "output", "o", "auto", "table|json|jsonl|csv|tsv|yaml|url|raw")
	pf.StringVar(&app.fields, "fields", "", "comma-separated columns to keep and order")
	pf.BoolVar(&app.noHeader, "no-header", false, "omit the header row")
	pf.StringVar(&app.template, "template", "", "Go text/template applied per record")
	pf.IntVarP(&app.limit, "limit", "n", 0, "max records emitted (0 = unlimited)")
	pf.StringVar(&app.cookie, "cookie", "", "cookie header (web_session=...; a1=...)")
	pf.StringVar(&app.cookieFile, "cookie-file", "", "path to a cookie file")
	pf.DurationVar(&app.rate, "rate", 600*time.Millisecond, "min delay between requests")
	pf.IntVar(&app.retries, "retries", 3, "retry attempts on 429/5xx/anti-bot")
	pf.DurationVar(&app.timeout, "timeout", 30*time.Second, "per-request timeout")
	pf.BoolVar(&app.cache, "cache", true, "use the on-disk response cache")
	pf.BoolVar(&app.noCache, "no-cache", false, "bypass the on-disk cache")
	pf.DurationVar(&app.cacheTTL, "cache-ttl", time.Hour, "cache freshness window")
	pf.BoolVarP(&app.quiet, "quiet", "q", false, "suppress progress on stderr")
	pf.CountVarP(&app.verbose, "verbose", "v", "increase verbosity (repeatable)")
	pf.StringVar(&app.proxy, "proxy", "", "HTTP or SOCKS proxy URL")
	pf.StringVar(&app.userAgent, "user-agent", "", "override the default desktop UA")
	pf.BoolVar(&app.raw, "raw", false, "print each record as pretty-printed JSON")
	pf.BoolVar(&app.dryRun, "dry-run", false, "print the requests that would be made")

	root.AddCommand(
		newNoteCmd(app),
		newUserCmd(app),
		newSearchCmd(app),
		newCommentsCmd(app),
		newFeedCmd(app),
		newRelatedCmd(app),
		newTagCmd(app),
		newSuggestCmd(app),
		newMeCmd(app),
		newIDCmd(app),
		newSessionCmd(app),
		newCrawlCmd(app),
		newConfigCmd(app),
		newCacheCmd(app),
		newVersionCmd(app),
	)
	return root
}

// Client lazily builds the xiaohongshu client from the resolved flags.
func (a *App) Client() *xiaohongshu.Client {
	if a.client != nil {
		return a.client
	}
	cfg := xiaohongshu.DefaultConfig()
	cfg.Rate = a.rate
	cfg.Retries = a.retries
	cfg.Timeout = a.timeout
	cfg.CacheTTL = a.cacheTTL
	cfg.NoCache = a.noCache || !a.cache
	cfg.DryRun = a.dryRun
	cfg.Proxy = firstNonEmpty(a.proxy, os.Getenv("XHS_PROXY"))
	cfg.UserAgent = firstNonEmpty(a.userAgent, os.Getenv("XHS_USER_AGENT"), xiaohongshu.DefaultUserAgent)
	cfg.Cookie = a.resolveCookie()
	a.client = xiaohongshu.NewClient(cfg)
	return a.client
}

func (a *App) resolveCookie() string {
	if a.cookie != "" {
		return a.cookie
	}
	if c := os.Getenv("XHS_COOKIE"); c != "" {
		return c
	}
	file := firstNonEmpty(a.cookieFile, os.Getenv("XHS_COOKIE_FILE"))
	if file != "" {
		if b, err := os.ReadFile(file); err == nil {
			return parseCookieFile(string(b))
		}
	}
	return ""
}

func parseCookieFile(s string) string {
	var parts []string
	for line := range strings.SplitSeq(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if f := strings.Split(line, "\t"); len(f) == 7 {
			parts = append(parts, f[5]+"="+f[6])
			continue
		}
		parts = append(parts, line)
	}
	return strings.Join(parts, "; ")
}

// resolveFormat turns "auto" into table on a tty or jsonl into a pipe.
func (a *App) resolveFormat() Format {
	f := a.output
	if v := os.Getenv("XHS_OUTPUT"); v != "" && f == "auto" {
		f = v
	}
	if a.raw {
		return FormatRaw
	}
	if Format(f) == FormatAuto || f == "" {
		if isatty.IsTerminal(os.Stdout.Fd()) {
			return FormatTable
		}
		return FormatJSONL
	}
	return Format(f)
}

func (a *App) newOutput() (*Output, error) {
	var fields []string
	if a.fields != "" {
		fields = strings.Split(a.fields, ",")
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
	}
	out, err := NewOutput(os.Stdout, a.resolveFormat(), fields, a.noHeader, a.template)
	if err != nil {
		return nil, err
	}
	out.suppress = a.dryRun
	return out, nil
}

func (a *App) progress(format string, args ...any) {
	if a.quiet {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func (a *App) ctx() context.Context {
	if a.rootCtx != nil {
		return a.rootCtx
	}
	return context.Background()
}

// readArgsOrStdin returns args, or lines from stdin when the single arg is "-".
func readArgsOrStdin(args []string) []string {
	if len(args) == 1 && args[0] == "-" {
		var out []string
		sc := bufio.NewScanner(os.Stdin)
		sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" {
				out = append(out, line)
			}
		}
		return out
	}
	return args
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
