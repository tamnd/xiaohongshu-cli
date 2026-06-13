---
title: "xhs"
description: "A command line for Xiaohongshu."
heroTitle: "xiaohongshu, from the command line"
heroLead: "A command line for Xiaohongshu. One pure-Go binary, no API key, output that pipes into the rest of your tools."
heroPrimaryURL: "/getting-started/quick-start/"
heroPrimaryText: "Get started"
---

A command line for Xiaohongshu. It reads public data from xiaohongshu.com,
shapes it into clean records, and pipes into the rest of your tools.

```bash
xhs search 'latte art' -n 5
xhs search 'latte art' -o url | xhs note -
xhs user <id> --notes -n 50
xhs feed --category food -n 40
```

It signs its own requests and bootstraps an anonymous web session. The deep
surfaces need a logged-in cookie and a residential IP; the [quick start](/getting-started/quick-start/)
covers what is reachable without one.

## Where to go next

- New here? Read the [introduction](/getting-started/introduction/), then the
  [quick start](/getting-started/quick-start/).
- Installing? See [installation](/getting-started/installation/).
- Need every flag? The [CLI reference](/reference/cli/) is the full surface.
