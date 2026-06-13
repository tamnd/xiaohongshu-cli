---
title: "Introduction"
description: "What xhs is and how it is put together."
weight: 10
---

A command line for Xiaohongshu.

xhs is a single binary. It speaks to xiaohongshu over plain HTTPS,
shapes the responses into clean records, and gets out of your way. There is
nothing to sign up for and nothing to run alongside it.

## How it is built

- A **library package** (`xiaohongshu`) holds the HTTP client and the typed
  data models. It paces requests, sets an honest User-Agent, and retries the
  transient failures any public site throws under load.
- A **command tree** (`cli`) wraps the library in subcommands with shared
  output formats and flags.
- One **`cmd/xhs`** entry point ties them together.
- The library also carries the parts specific to Xiaohongshu: a request signer
  (`pkg/xhssign`), an anonymous web session bootstrap, a server-rendered page
  reader (`pkg/xhshtml`), and the id, url, and xsec_token parser (`pkg/xhsurl`).

## What works without a cookie

`note` and `feed` work anonymously from any IP. `user`, `user --notes`, and
`related` work on a fresh IP at a slow pace and want a cookie for heavy use.
`comments`, `search`, `suggest`, `tag`, and `me` need a logged-in cookie. Pass
one with `--cookie` or `XHS_COOKIE`.

## Scope

xhs is a read-only client over data xiaohongshu already serves publicly. It reads
that data and shapes it for you. That narrow scope keeps it a single small binary
with no database, no daemon, and no setup. The `crawl` command walks outward from
the feed or seed notes and streams connected records to JSONL files. See the
[commands](/reference/commands/) page for every command with sample input and
output, and the [quick start](/getting-started/quick-start/) to get going.

Next: [install it](/getting-started/installation/), then take the
[quick start](/getting-started/quick-start/).
