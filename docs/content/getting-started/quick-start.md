---
title: "Quick start"
description: "Run your first xhs command."
weight: 30
---

Once `xhs` is on your `PATH`:

```bash
xhs --help       # see the command tree
xhs version      # build info
```

When stdout is a pipe, `xhs` prints JSONL, one record per line, so output feeds
`jq` or another `xhs` command with no flags. When stdout is a terminal it prints
a table. Choose a format with `-o table|json|jsonl|csv|tsv|yaml|url|raw`.

## A few commands

```bash
# search notes, then open the first result
xhs search 'latte art' -n 5
xhs search 'latte art' -o url | xhs note -

# a creator profile and their notes
xhs user 5ff0e6500000000001008400
xhs user <id> --notes -n 50

# a note's comments with replies
xhs comments <note-id> --token <xsec_token> --deep -n 100

# the recommendation homefeed
xhs feed --category food -n 40
```

## Opening a note needs a token

A note will not open without its `xsec_token`. You get one from a search result,
a listing, or a share URL. `xhs id` pulls the id and token out of any link:

```bash
xhs id 'https://www.xiaohongshu.com/explore/<id>?xsec_token=<t>&xsec_source=pc_feed'
```

## Gated surfaces need a cookie

Xiaohongshu blocks datacenter IPs and rate-limits hard, and the personalized
surfaces need a login. Run from a residential IP at the default polite pace, and
pass a real cookie for login-gated commands:

```bash
export XHS_COOKIE='web_session=...; a1=...'
xhs me
```

See [configuration](/reference/configuration/) for every variable.
