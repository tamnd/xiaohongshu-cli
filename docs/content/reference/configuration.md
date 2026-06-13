---
title: "Configuration"
description: "Environment variables and global flags."
weight: 20
---

Flags win over environment variables, which win over defaults. `xhs config show`
prints the resolved settings with the cookie redacted; `xhs config path` prints
the directories.

## Environment variables

| Variable | Meaning |
|---|---|
| `XHS_COOKIE` | cookie header for login-gated surfaces |
| `XHS_COOKIE_FILE` | path to a cookie file (header line or Netscape format) |
| `XHS_PROXY` | HTTP or SOCKS proxy URL |
| `XHS_USER_AGENT` | override the default desktop UA |
| `XHS_OUTPUT` | default output format when `-o` is `auto` |
| `XHS_CACHE_DIR` | cache location (default: OS cache dir + `/xhs`) |
| `XHS_CONFIG_DIR` | config and session location (default: OS config dir + `/xhs`) |

## Cookies and the session

The anonymous session (the `a1` cookie) is bootstrapped on first use and cached
under the config dir as `session.json`. Inspect or reset it:

```bash
xhs session show
xhs session forget
```

Login-gated surfaces (the personalized homefeed, a user's liked or collected
notes, the `me` check) need a real `web_session` cookie copied from a browser:

```bash
export XHS_COOKIE='web_session=...; a1=...'
```

## The cache

Responses are cached on disk keyed by the signed request. Manage it with:

```bash
xhs cache stat     # location, file count, size
xhs cache clear    # delete every cached response
xhs cache path     # print the directory
```

Bypass it for a single run with `--no-cache`, or set the freshness window with
`--cache-ttl`.
