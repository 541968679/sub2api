#!/bin/bash
# Phase-1 local smoke. Run on hk-relay as root. Does not change sshd/ufw.
set -euo pipefail
HOST_HDR='Host: 191.40.32.186'
BASE='http://127.0.0.1'

echo '=== /v1/models ==='
code=$(curl -sS -o /tmp/models.body -w '%{http_code}' -m 25 -H "$HOST_HDR" "$BASE/v1/models")
echo "status=$code"
head -c 200 /tmp/models.body
echo
test "$code" = 401

echo '=== /api/v1/settings/public ==='
code=$(curl -sS -o /tmp/pub.body -w '%{http_code}' -m 25 -H "$HOST_HDR" "$BASE/api/v1/settings/public")
echo "status=$code bytes=$(wc -c < /tmp/pub.body)"
test "$code" = 200

echo '=== / ==='
code=$(curl -sS -o /tmp/root.body -w '%{http_code}' -m 25 -H "$HOST_HDR" "$BASE/")
echo "status=$code bytes=$(wc -c < /tmp/root.body)"
test "$code" = 200

echo '=== 2MB POST /v1/chat/completions ==='
code=$(dd if=/dev/zero bs=1M count=2 status=none | curl -sS -o /tmp/post.body -w '%{http_code}' -m 40 -X POST -H "$HOST_HDR" -H 'Content-Type: application/json' --data-binary @- "$BASE/v1/chat/completions")
echo "status=$code"
head -c 200 /tmp/post.body
echo
test "$code" != 413

echo '=== ~10MB download ==='
bash "$(dirname "$0")/download-10mb.sh"
echo OK
