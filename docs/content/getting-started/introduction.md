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
  (`pkg/xhssign`), an anonymous web session bootstrap, the `__INITIAL_STATE__`
  extractor for server-rendered pages (`pkg/xhshtml`), and the id, url, and
  xsec_token parser (`pkg/xhsurl`).

## How it reads data

Xiaohongshu serves each page twice over: the server renders it once with the data
already embedded in a `window.__INITIAL_STATE__` script, then the browser keeps it
fresh over a signed JSON API. The signed API refuses anonymous callers, so xhs
reads the server-rendered state first and only falls back to the signed API when
you give it a logged-in cookie.

That makes **note** and **feed** work anonymously from any IP. The profile-derived
surfaces (**user**, **user --notes**, **related**) read the server-rendered
profile page, which Xiaohongshu rate-limits hard per IP, so they work on a fresh
IP at a slow pace and want a cookie for sustained crawling. **comments**,
**search**, **suggest**, **tag**, and **me** are only ever loaded over the signed
API, so they need a cookie.

## Scope

xhs is a read-only client over data xiaohongshu already serves publicly. It reads
that data and shapes it for you. That narrow scope keeps it a single small binary
with no database, no daemon, and no setup. The `crawl` command walks outward from
the feed or seed notes and streams connected records to JSONL files. See the
[quick start](/getting-started/quick-start/) and
[configuration](/reference/configuration/) for the details.

Next: [install it](/getting-started/installation/), then take the
[quick start](/getting-started/quick-start/).
