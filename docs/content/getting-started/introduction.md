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
  (`pkg/xhssign`), an anonymous web session bootstrap, and the id, url, and
  xsec_token parser (`pkg/xhsurl`).

## Scope

xhs is a read-only client over data xiaohongshu already serves publicly. It reads
that data and shapes it for you. That narrow scope keeps it a single small binary
with no database, no daemon, and no setup.

Xiaohongshu guards its web API: it blocks datacenter IPs and rate-limits hard, so
run xhs from a residential connection at the default polite pace, and pass a
cookie for the login-gated surfaces. See the [quick start](/getting-started/quick-start/)
and [configuration](/reference/configuration/) for the details.

Next: [install it](/getting-started/installation/), then take the
[quick start](/getting-started/quick-start/).
