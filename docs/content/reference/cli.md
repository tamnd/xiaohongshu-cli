---
title: "CLI"
description: "Every command and subcommand, with the flags that matter."
weight: 10
---

```
xhs <command> [subcommand] [flags]
```

Run `xhs <command> --help` for the full flag list on any command.

## Commands

| Command | What it does |
|---|---|
| `note <id\|url>...` | Resolve one or more notes to full metadata. `--token` supplies the xsec_token. |
| `user <id\|url>...` | Resolve a creator profile, or list their notes with `--notes`. |
| `search <keyword>` | Search notes, or users with `--users`. |
| `comments <note-id\|url>` | Stream a note's comments. `--deep` includes replies, `--token` supplies the xsec_token. |
| `feed` | Stream the recommendation homefeed. `--category` picks a channel, `--list` shows the channels. |
| `related <note-id\|url>` | List notes recommended alongside a note. |
| `tag <keyword>` | Resolve a topic page to its name, id, and view count. |
| `suggest <keyword>` | Print search autocomplete suggestions. |
| `me` | Show the login state of the configured cookie. |
| `id <input>...` | Parse ids, urls, and xsec_tokens out of any xiaohongshu link. |
| `session show\|forget` | Inspect or reset the persisted anonymous web session. |
| `crawl <id\|url>...` | Crawl connected records from seed notes into JSONL files. |
| `config show\|path` | Show resolved configuration and paths. |
| `cache stat\|clear\|path` | Manage the on-disk response cache. |
| `version` | Print the version and exit. |

## Global flags

These apply to every command.

| Flag | Default | Meaning |
|---|---|---|
| `-o, --output` | `auto` | `table\|json\|jsonl\|csv\|tsv\|yaml\|url\|raw` |
| `--fields` | | comma-separated columns to keep and order |
| `--template` | | Go `text/template` applied per record |
| `--no-header` | `false` | omit the header row in table and csv |
| `-n, --limit` | `0` | max records emitted (0 is unlimited) |
| `--cookie` | | cookie header for gated surfaces |
| `--cookie-file` | | path to a cookie file |
| `--rate` | `600ms` | minimum delay between requests |
| `--retries` | `3` | retry attempts on 429, 5xx, or anti-bot |
| `--timeout` | `30s` | per-request timeout |
| `--cache` / `--no-cache` | on | use or bypass the on-disk cache |
| `--cache-ttl` | `1h` | cache freshness window |
| `--proxy` | | HTTP or SOCKS proxy URL |
| `--user-agent` | | override the default desktop UA |
| `--raw` | `false` | print each record as pretty JSON |
| `--dry-run` | `false` | print the requests that would be made |
| `-q, --quiet` | `false` | suppress progress on stderr |

## Exit codes

| Code | Meaning |
|---|---|
| `0` | success |
| `3` | the surface needs a login cookie |
| `4` | not found |
| `5` | rate-limited or walled by anti-bot |
| `6` | network error |
| `1` | any other error |
