---
title: "Release notes"
linkTitle: "Release notes"
description: "What changed in each xhs release, newest first."
weight: 40
---

What shipped in each release, newest first. Every tagged version builds the same
set of artifacts: archives for Linux, macOS, Windows, and FreeBSD, Linux
packages (deb, rpm, apk), a multi-arch container image on GHCR, and entries for
the package managers. Binaries are pure Go, so there is nothing to install
alongside them.

- [v0.2.0](/release-notes/v0-2-0/) reads public data without a cookie from the
  server-rendered page, fixes the profile login wall, makes the exit codes
  honest, grows `crawl` into a small scraping engine, and rewrites the docs
  around how to use each command.
- [v0.1.0](/release-notes/v0-1-0/) is the first release: notes, users, comments,
  search, the homefeed, related notes, topics, and crawl, with the request
  signer and anonymous session reimplemented from scratch.
