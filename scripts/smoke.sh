#!/usr/bin/env bash
# Exercise every xhs command against the live xiaohongshu.com API and report a
# pass/fail line per command. Read-only: it only fetches public data, never logs
# in, downloads media, or writes anything outside a temp directory.
#
# Usage:
#   ./scripts/smoke.sh              # uses the xhs on $PATH
#   XHS=./bin/xhs ./scripts/smoke.sh
#   XHS_COOKIE='web_session=...; a1=...' ./scripts/smoke.sh   # gated surfaces too
#
# The note and feed surfaces read the server-rendered page (__INITIAL_STATE__)
# and work anonymously from any IP, so they are expected to PASS without a
# cookie. The profile-derived surfaces (user, user-notes, related) also read the
# server-rendered profile page, but Xiaohongshu rate-limits that page hard per
# IP: a cold IP serves it, then it redirects to login for a cooldown window.
# Those pass when the IP is cold and SKIP when it is hot. Comments and search are
# only ever loaded over the signed JSON API, which refuses anonymous callers, so
# they need a logged-in cookie and are reported as SKIP. None of the SKIPs are a
# code fault: they need a residential IP or a cookie, not a code change.

set -u

XHS="${XHS:-xhs}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

pass=0 fail=0 skip=0

walled() {
  grep -qi 'anti-bot\|risk control\|too many requests\|461\|406\|300012\|300013\|300015\|-100\|-101\|needs a login\|request failed after retries\|HTTP 4' "$1"
}

# run NAME -- args... : succeeds when the command exits 0 and prints something.
# Retries once after a pause so a single transient rate-limit does not fail the
# suite.
run() {
  local name="$1"; shift
  [ "$1" = "--" ] && shift
  local out rc
  out="$("$XHS" "$@" 2>"$TMP/err")"; rc=$?
  if [ $rc -ne 0 ] || [ -z "$out" ]; then
    sleep 3
    out="$("$XHS" "$@" 2>"$TMP/err")"; rc=$?
  fi
  if [ $rc -eq 0 ] && [ -n "$out" ]; then
    printf 'PASS  %-22s %s\n' "$name" "$*"
    pass=$((pass + 1))
    printf '%s\n' "$out" | head -1 | cut -c1-100
  elif walled "$TMP/err"; then
    printf 'SKIP  %-22s %s  (anti-bot / needs cookie)\n' "$name" "$*"
    skip=$((skip + 1))
  else
    printf 'FAIL  %-22s %s\n' "$name" "$*"
    sed 's/^/      /' "$TMP/err" | head -3
    fail=$((fail + 1))
  fi
  sleep 0.8
}

echo "xhs smoke test against the live API"
echo "binary: $XHS"
"$XHS" version
echo

NOTE_ID="6849c2f0000000001e034c8e"
USER_ID="5ff0e6500000000001008400"

# --- offline: parsing and meta, never touch the network ---
run id                 -- id "https://www.xiaohongshu.com/explore/$NOTE_ID?xsec_token=ABC123&xsec_source=pc_feed" -o jsonl
run id-user            -- id "https://www.xiaohongshu.com/user/profile/$USER_ID" -o jsonl
run feed-list          -- feed --list -o jsonl
run config-show        -- config show -o jsonl
run config-path        -- config path -o jsonl
run cache-stat         -- cache stat -o jsonl
run session-show       -- session show -o jsonl
run version-json       -- version -o jsonl

# Harvest a real note, token, and author from the explore feed. The feed is read
# from the server-rendered page, so this works anonymously and seeds the note,
# related, and user checks with live ids and a fresh token.
SEED="$("$XHS" feed -n 1 -o jsonl 2>/dev/null | head -1)"
HNOTE="$(printf '%s' "$SEED" | sed -n 's/.*"note_id":"\([^"]*\)".*/\1/p')"
HTOKEN="$(printf '%s' "$SEED" | sed -n 's/.*"xsec_token":"\([^"]*\)".*/\1/p')"
HUSER="$(printf '%s' "$SEED" | sed -n 's/.*"user_id":"\([^"]*\)".*/\1/p')"
HNOTE="${HNOTE:-$NOTE_ID}"
HUSER="${HUSER:-$USER_ID}"

# --- anonymous server-rendered surfaces (expected to pass without a cookie) ---
run feed               -- feed --category food -n 3 -o jsonl
if [ -n "$HTOKEN" ]; then
  run note             -- note "$HNOTE" --token "$HTOKEN" -o jsonl
else
  printf 'SKIP  %-22s note  (no token harvested, IP walled)\n' "note"
  skip=$((skip + 1))
fi

# --- profile-derived surfaces (anonymous on a cold IP, rate-walled when hot) ---
run user               -- user "$HUSER" -o jsonl
run user-notes         -- user "$HUSER" --notes -n 3 -o jsonl
if [ -n "$HTOKEN" ]; then
  run related          -- related "$HNOTE" --token "$HTOKEN" -n 3 -o jsonl
else
  printf 'SKIP  %-22s related  (no token harvested)\n' "related"
  skip=$((skip + 1))
fi

# --- signed-API-only surfaces (need a logged-in cookie, SKIP anonymously) ---
run suggest            -- suggest coffee
run search-notes       -- search 'latte art' -n 3 -o jsonl
run search-users       -- search coffee --users -n 2 -o jsonl
run tag                -- tag coffee -o jsonl
if [ -n "$HTOKEN" ]; then
  run comments         -- comments "$HNOTE" --token "$HTOKEN" -n 3 -o jsonl
else
  printf 'SKIP  %-22s comments  (no token harvested)\n' "comments"
  skip=$((skip + 1))
fi

# --- login-gated (needs XHS_COOKIE) ---
if [ -n "${XHS_COOKIE:-}" ]; then
  run me               -- me -o jsonl
else
  printf 'SKIP  %-22s me  (set XHS_COOKIE)\n' "me"
  skip=$((skip + 1))
fi

echo
echo "----------------------------------------"
echo "pass=$pass  skip=$skip  fail=$fail"
[ $fail -eq 0 ]
