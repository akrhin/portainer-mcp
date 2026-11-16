#!/usr/bin/env bash
# Drives the built server over stdio: initialize, tools/list, then every core
# tool. Fails if any call reports isError or a reply is missing.
set -euo pipefail
cd "$(dirname "$0")/.."

config="${1:-config.local.yaml}"
env_id="${SMOKE_ENV_ID:-1}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

go build -o "$tmp/portainer-mcp" ./cmd/portainer-mcp

cat >"$tmp/in.jsonl" <<EOF
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2026-07-28","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"system_version","arguments":{}}}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"system_status","arguments":{}}}
{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"system_info","arguments":{}}}
{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"settings_get","arguments":{}}}
{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"whoami","arguments":{}}}
{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"environment_list","arguments":{}}}
{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"environment_inspect","arguments":{"id":$env_id}}}
EOF

"$tmp/portainer-mcp" --config "$config" >"$tmp/out.jsonl" < <(cat "$tmp/in.jsonl"; sleep 5)

replies="$(grep -c '"jsonrpc"' "$tmp/out.jsonl" || true)"
tools="$(grep -o '"name":"[a-z_]*"' "$tmp/out.jsonl" | sort -u | wc -l)"
echo "replies: $replies (want 9)"
echo "tool names seen: $tools"
if grep -q '"isError":true' "$tmp/out.jsonl"; then
  echo "FAIL: a tool call returned isError:" >&2
  grep -o '"text":"[^"]\{0,200\}' "$tmp/out.jsonl" >&2
  exit 1
fi
if [ "$replies" -ne 9 ]; then
  echo "FAIL: missing replies" >&2
  cat "$tmp/out.jsonl" >&2
  exit 1
fi
echo "OK: server served all core tools"
