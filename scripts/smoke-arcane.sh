#!/usr/bin/env bash
# Drives the Arcane server over stdio: initialize, tools/list, then a handful of
# read-only calls against one agent. Fails if any call reports isError.
#
#   ARCANE_URL=http://arcane:3552 ARCANE_API_KEY=... [SMOKE_AGENT_ID=0] \
#     [SMOKE_TOOLSETS=core,containers,projects,images,volumes,networks] scripts/smoke-arcane.sh
set -euo pipefail
cd "$(dirname "$0")/.."

url="${ARCANE_URL:-}"
[ -n "$url" ] || { echo "ARCANE_URL is required" >&2; exit 2; }
[ -n "${ARCANE_API_KEY:-}" ] || { echo "ARCANE_API_KEY is required" >&2; exit 2; }
agent="${SMOKE_AGENT_ID:-0}"
toolsets="${SMOKE_TOOLSETS:-core,containers,projects,images,volumes,networks}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

go build -o "$tmp/arcane-mcp" ./cmd/arcane-mcp

in="$tmp/in.jsonl"
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2026-07-28","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' >"$in"

id=2
probes=""
call() { # call <tool> <arguments-json>
  id=$((id + 1))
  probes="$probes $id:$1"
  printf '{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"%s","arguments":%s}}\n' \
    "$id" "$1" "$2" >>"$in"
}

call environments_list '{}'
call environment_inspect "{\"id\":\"$agent\"}"
call containers_list "{\"id\":\"$agent\"}"
call container_status_count "{\"id\":\"$agent\"}"
call projects_list "{\"id\":\"$agent\"}"
call image_update_summary "{\"id\":\"$agent\"}"
call volumes_list "{\"id\":\"$agent\"}"
call networks_list "{\"id\":\"$agent\"}"

"$tmp/arcane-mcp" --url "$url" --api-key "$ARCANE_API_KEY" --toolsets "$toolsets" \
  >"$tmp/out.jsonl" < <(cat "$in"; sleep 15)

reply() { grep -F "\"id\":$1," "$tmp/out.jsonl" || true; }

replies="$(grep -c '"jsonrpc"' "$tmp/out.jsonl" || true)"
echo "agent: $agent, toolsets: $toolsets"
echo "replies: $replies (want $id, ids 1..$id)"
echo "tools registered: $(grep -o '"name":"[a-z_]*"' "$tmp/out.jsonl" | sort -u | wc -l)"

fail=0
for probe in $probes; do
  pid="${probe%%:*}"
  pname="${probe#*:}"
  line="$(reply "$pid")"
  if [ -z "$line" ]; then
    echo "FAIL: $pname did not answer" >&2
    fail=1
  elif printf '%s' "$line" | grep -q '"isError":true'; then
    echo "FAIL: $pname: $(printf '%s' "$line" | cut -c1-300)" >&2
    fail=1
  else
    echo "ok: $pname"
  fi
done
if [ "$fail" -ne 0 ]; then
  exit 1
fi
if [ "$replies" -ne "$id" ]; then
  echo "FAIL: missing replies" >&2
  cat "$tmp/out.jsonl" >&2
  exit 1
fi
echo "OK: every probe answered"
