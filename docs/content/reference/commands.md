---
title: "Commands"
description: "Every command with sample input and output."
weight: 5
---

Each command below shows the call and a sample of what it prints. Output is JSONL
(one record per line) when stdout is a pipe, which is what these samples show. In
a terminal you get a table instead; pick any format with `-o`.

The fields are trimmed here for width. Add `--raw` to see a record as pretty JSON,
or `--fields a,b,c` to keep just the columns you want.

## feed

Stream the explore homefeed. Works without a cookie. This is the easiest way to
get note ids and their `xsec_token`.

```bash
xhs feed -n 2
xhs feed --category food -n 40      # a named channel
xhs feed --list                     # the channel names
```

```jsonl
{"note_id":"647da4f8000000001300d8d0","type":"normal","title":"正确防水步骤！","user_id":"643ca19d000000000e01d0fd","nickname":"装修日记","cover":"http://sns-webpic-qc.xhscdn.com/...","liked_count":4704,"xsec_token":"ABi__3DZQqC...","url":"https://www.xiaohongshu.com/explore/647da4f8000000001300d8d0?xsec_token=ABi__3DZQqC...","fetched_at":"2026-06-14T00:12:02Z"}
{"note_id":"6460c612000000002702a7fc","type":"video","title":"母亲节这顿香出新高度","user_id":"5930d6a882ec396d3c6a6764","nickname":"我是晴天","cover":"http://sns-webpic-qc.xhscdn.com/...","liked_count":70000,"xsec_token":"ABPvi0oh9iU...","url":"https://www.xiaohongshu.com/explore/6460c612000000002702a7fc?xsec_token=ABPvi0oh9iU...","fetched_at":"2026-06-14T00:12:02Z"}
```

`--list`:

```jsonl
{"category":"recommend"}
{"category":"food"}
{"category":"fashion"}
```

## note

Resolve one or more notes to full metadata. Works without a cookie. A note needs
its `xsec_token`, which travels with it in the feed and in share URLs.

```bash
xhs note 647da4f8000000001300d8d0 --token ABi__3DZQqC...
xhs note 'https://www.xiaohongshu.com/explore/<id>?xsec_token=<t>&xsec_source=pc_feed'
xhs feed -o url | xhs note -          # pipe ids in
```

```jsonl
{"note_id":"647da4f8000000001300d8d0","type":"video","title":"谁懂啊 真的好喜欢这种感觉","desc":"真的好爱重庆的盖碗茶火锅！#重庆美食[话题]#","user_id":"643ca19d000000000e01d0fd","nickname":"装修日记","liked_count":51000,"collected_count":12000,"comment_count":834,"share_count":2100,"time":1700000000000,"time_text":"2025-11-14T22:13:20Z","ip_location":"重庆","tags":["重庆美食","重庆旅游"],"video":{"duration_seconds":42,"width":720,"height":1280,"masters":["http://sns-video-bd.xhscdn.com/..."]},"xsec_token":"ABi__3DZQqC...","url":"https://www.xiaohongshu.com/explore/647da4f8000000001300d8d0","fetched_at":"2026-06-14T00:12:05Z"}
```

## user

Resolve a creator profile, or list their notes with `--notes`. Works on a fresh
IP at a slow pace; use a cookie for heavy use.

```bash
xhs user 5ba0c7223fa1ad0001dde6b0
xhs user 5ba0c7223fa1ad0001dde6b0 --notes -n 50
```

Profile:

```jsonl
{"user_id":"5ba0c7223fa1ad0001dde6b0","nickname":"一颗咖啡豆","red_id":"95271864","desc":"每天一杯手冲","gender":"female","ip_location":"上海","follows":214,"fans":52000,"interaction":830000,"tags":["美食","咖啡"],"url":"https://www.xiaohongshu.com/user/profile/5ba0c7223fa1ad0001dde6b0","fetched_at":"2026-06-14T00:12:08Z"}
```

`--notes`:

```jsonl
{"note_id":"66f1a0c2000000001e034c8e","type":"normal","title":"在家复刻一杯dirty","user_id":"5ba0c7223fa1ad0001dde6b0","nickname":"一颗咖啡豆","cover":"http://sns-webpic-qc.xhscdn.com/...","liked_count":3100,"xsec_token":"ABk22f...","url":"https://www.xiaohongshu.com/explore/66f1a0c2000000001e034c8e?xsec_token=ABk22f...","fetched_at":"2026-06-14T00:12:08Z"}
```

## related

List notes recommended alongside a note. Reads the author's other posts without a
cookie.

```bash
xhs related 647da4f8000000001300d8d0 --token ABi__3DZQqC... -n 3
```

```jsonl
{"note_id":"66aa12...","type":"video","title":"重庆三日游全攻略","user_id":"643ca19d000000000e01d0fd","nickname":"装修日记","liked_count":8900,"xsec_token":"ABcd...","url":"https://www.xiaohongshu.com/explore/66aa12...?xsec_token=ABcd...","fetched_at":"2026-06-14T00:12:10Z"}
```

## search

Search notes, or users with `--users`. Needs a cookie.

```bash
xhs search 'latte art' -n 5
xhs search 'travel japan' --users -n 5
```

```jsonl
{"note_id":"65c0f1...","type":"normal","title":"latte art 拉花练习","user_id":"5e8f...","nickname":"咖啡师小李","liked_count":1200,"xsec_token":"ABef...","url":"https://www.xiaohongshu.com/explore/65c0f1...","fetched_at":"2026-06-14T00:12:12Z"}
```

## comments

Stream a note's comments. `--deep` includes replies. Needs a cookie.

```bash
xhs comments 647da4f8000000001300d8d0 --token ABi__3DZQqC... -n 3
xhs comments 647da4f8000000001300d8d0 --token ABi__3DZQqC... --deep
```

```jsonl
{"comment_id":"66f2...","note_id":"647da4f8000000001300d8d0","content":"看着就好吃","user_id":"5d3a...","nickname":"吃货本货","like_count":42,"sub_comment_count":2,"ip_location":"四川","create_time":1700001234000,"fetched_at":"2026-06-14T00:12:15Z"}
```

## tag

Resolve a topic page to its name, id, and view count. Needs a cookie.

```bash
xhs tag coffee
```

```jsonl
{"id":"5be35...","name":"coffee","type":"topic","view_num":120000000,"link":"https://www.xiaohongshu.com/page/topics/5be35...","fetched_at":"2026-06-14T00:12:17Z"}
```

## suggest

Print search autocomplete suggestions. Needs a cookie.

```bash
xhs suggest cof
```

```jsonl
{"term":"coffee"}
{"term":"coffee bean"}
{"term":"coffee shop 上海"}
```

## id

Parse ids, urls, and xsec_tokens out of any xiaohongshu link. Pure parsing, no
network.

```bash
xhs id 'https://www.xiaohongshu.com/explore/647da4f8000000001300d8d0?xsec_token=ABi__3DZQqC&xsec_source=pc_feed'
```

```jsonl
{"kind":"note","note_id":"647da4f8000000001300d8d0","user_id":"","xsec_source":"pc_feed","xsec_token":"ABi__3DZQqC"}
```

## me

Show the login state of the configured cookie.

```bash
xhs me --cookie 'web_session=...; a1=...'
```

```jsonl
{"logged_in":true,"user_id":"5ff0e6...","nickname":"我的账号","red_id":"12345678"}
```

## crawl

Walk outward from the feed or seed notes and stream connected records to JSONL
files in `--out`. See [crawling](#crawl-flags) below for the full flag list.

```bash
# seed from the feed, walk two hops, cap at 500 notes
xhs crawl --explore --depth 2 --max 500 --out ./data

# a category, following each author's other notes
xhs crawl --category food --author-notes --out ./food

# specific notes with comments and related
xhs crawl 647da4f8000000001300d8d0 --token ABi__3DZQqC --comments --out ./data
```

It writes `notes.jsonl`, `users.jsonl`, and `comments.jsonl` as records are
found, and prints progress on stderr:

```text
seeded 30 notes from explore/food
note 647da4f8000000001300d8d0 "谁懂啊 真的好喜欢这种感觉" (depth 0, 1 done, 29 queued)
note 6460c612000000002702a7fc "母亲节这顿香出新高度" (depth 0, 2 done, 31 queued)
crawl done: 412 notes, 188 users into ./data
```

```jsonl
$ head -1 ./data/notes.jsonl
{"note_id":"647da4f8000000001300d8d0","type":"video","title":"谁懂啊 真的好喜欢这种感觉","user_id":"643ca19d000000000e01d0fd","nickname":"装修日记","liked_count":51000,"url":"https://www.xiaohongshu.com/explore/647da4f8000000001300d8d0","fetched_at":"2026-06-14T00:12:20Z"}
```

### Crawl flags

| Flag | Default | Meaning |
|---|---|---|
| `--out` | `.` | output directory for the JSONL files |
| `--explore` | `false` | seed the frontier from the explore feed |
| `--category` | | seed from a named explore category (implies `--explore`) |
| `--seed-limit` | `30` | how many explore notes to seed with |
| `--depth` | `1` | how many hops to follow related and author notes |
| `--max` | `0` | stop after this many notes (0 means no cap) |
| `--related` | `true` | follow related notes into the frontier |
| `--author-notes` | `false` | follow each author's other notes into the frontier |
| `--user` | `true` | also crawl the note author profile |
| `--comments` | `false` | also crawl comments |
| `--deep` | `false` | include comment replies when crawling comments |
| `--token` | | xsec_token shared by the seed notes |

## Meta commands

```bash
xhs version                 # {"version":"v0.1.0","commit":"...","date":"..."}
xhs config show             # resolved settings, cookie redacted
xhs config path             # config, cache, and session paths
xhs cache stat              # {"dir":"...","files":128,"bytes":4194304,"size":"4.0 MB"}
xhs cache clear             # delete every cached response
xhs session show            # {"path":"...","exists":true}
xhs session forget          # drop the anonymous session, bootstrap a fresh one
```
