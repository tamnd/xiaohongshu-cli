# xhs

A command line for Xiaohongshu.

`xhs` is a single pure-Go binary. It reads public data from xiaohongshu.com over
plain HTTPS, shapes the responses into clean records, and pipes into the rest of
your tools. No paid API key, nothing to run alongside it. It signs its own
requests and bootstraps an anonymous web session the way a browser does.

## Install

```bash
go install github.com/tamnd/xiaohongshu-cli/cmd/xhs@latest
```

Or grab a prebuilt binary from the [releases](https://github.com/tamnd/xiaohongshu-cli/releases), or run
the container image:

```bash
docker run --rm ghcr.io/tamnd/xhs:latest --help
```

## Usage

When stdout is a pipe, `xhs` prints JSONL, one record per line, so a run feeds
`jq`, `awk`, or another `xhs` command with no flags. When stdout is a terminal it
prints a compact table. Pick a format yourself with `-o`.

```bash
# open a note (the xsec_token comes from a listing or a share URL)
xhs note 6849c2f0000000001e034c8e --token <xsec_token>
xhs note 'https://www.xiaohongshu.com/explore/<id>?xsec_token=<t>&xsec_source=pc_feed'

# a creator profile, or the creator's notes
xhs user 5ff0e6500000000001008400
xhs user <id> --notes -n 50

# search notes or users
xhs search 'latte art' -n 40
xhs search 'travel japan' --users

# a note's comments, optionally with replies
xhs comments <note-id> --token <t> --deep -n 100

# the recommendation homefeed
xhs feed --category food -n 40
xhs feed --list

# topics, related notes, autocomplete
xhs tag coffee
xhs related <note-id> --token <t>
xhs suggest cof

# parse ids, urls, and tokens out of any link
xhs id 'https://www.xiaohongshu.com/explore/<id>?xsec_token=<t>'
```

Pipe one command into the next. Every command that prints notes can emit just the
URL with `-o url`, and the next command reads ids from stdin with `-`:

```bash
xhs search coffee -o url | xhs note -
xhs search coffee -n 100 | xhs crawl - --out ./data --comments
```

### Output

`-o table|json|jsonl|csv|tsv|yaml|url|raw` picks the format. `--fields a,b,c`
keeps and orders columns. `--template '{{.note_id}} {{.title}}'` renders each
record with Go `text/template`. `-n` caps the record count. `--raw` prints each
record as pretty JSON.

## The anti-bot reality

Xiaohongshu guards its web API. It blocks datacenter IP ranges (most cloud and CI
hosts) within minutes and rate-limits every IP hard. From a residential
connection the public surfaces above are reachable at a polite pace. From a
server or a CI runner most calls come back as risk-control rejections, and the
deeper, personalized surfaces always need a logged-in cookie.

So:

- Run it from a residential IP, slowly. The default `--rate` is 600ms.
- Opening a note needs an `xsec_token`. You get one from a listing, a search
  result, or a share URL; it travels with the note and `xhs id` pulls it out.
- For login-gated surfaces (the personalized homefeed, a user's liked or
  collected notes, the `me` login check), pass a real cookie:

```bash
xhs me --cookie 'web_session=...; a1=...'
export XHS_COOKIE='web_session=...; a1=...'
```

The anonymous session (the `a1` cookie) is bootstrapped on first use and cached
under your config dir. Inspect or reset it with `xhs session show` and
`xhs session forget`.

## Configuration

Flags win over environment variables, which win over defaults.

| Variable | Meaning |
| --- | --- |
| `XHS_COOKIE` | cookie header for gated surfaces |
| `XHS_COOKIE_FILE` | path to a cookie file (header or Netscape format) |
| `XHS_PROXY` | HTTP or SOCKS proxy URL |
| `XHS_USER_AGENT` | override the default desktop UA |
| `XHS_OUTPUT` | default output format |
| `XHS_CACHE_DIR` | cache location |
| `XHS_CONFIG_DIR` | config and session location |

`xhs config show` prints the resolved settings with the cookie redacted.
`xhs cache stat|clear|path` manages the on-disk response cache.

Exit codes: `0` success, `3` needs a login, `4` not found, `5` rate-limited or
walled by anti-bot, `6` network error, `1` anything else.

## Development

```
cmd/xhs/       thin main, wires cli.Root into fang and maps exit codes
cli/           the cobra command tree and the output formatter
xiaohongshu/   the library: signed HTTP client, session, and data models
pkg/xhssign/   the request signer (x-s/x-t/x-s-common)
pkg/xhsurl/    the id, url, and xsec_token parser
docs/          tago documentation site
```

```bash
make build      # ./bin/xhs
make test       # go test ./...
make vet        # go vet ./...
```

## Releasing

Push a version tag and GitHub Actions runs GoReleaser, which builds the
archives, Linux packages, the multi-arch GHCR image, checksums, SBOMs, and a
cosign signature:

```bash
git tag v0.1.0
git push --tags
```

The Homebrew and Scoop steps self-disable until their tokens exist, so the first
release works with no extra secrets.

## License

Apache-2.0. See [LICENSE](LICENSE). The request signer is a clean-room
reimplementation built from observing the public web client; no third-party code
is vendored.
