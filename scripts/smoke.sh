#!/usr/bin/env bash
# Drives the built server over stdio: initialize, tools/list, then the curated
# core tools plus one read tool from every set in SMOKE_TOOLSETS. Fails if any
# call reports isError or a reply is missing.
set -euo pipefail
cd "$(dirname "$0")/.."

config="${1:-config.local.yaml}"
env_id="${SMOKE_ENV_ID:-1}"
toolsets="${SMOKE_TOOLSETS:-core}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

go build -o "$tmp/portainer-mcp" ./cmd/portainer-mcp

in="$tmp/in.jsonl"
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2026-07-28","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' >"$in"

id=2
probes=""
cores=""
streams=""
call() { # call <tool> <arguments-json> [probe|stream]
  id=$((id + 1))
  case "${3:-}" in
    probe) probes="$probes $id:$1" ;;
    stream) streams="$streams $id:$1" ;;
    *) cores="$cores $id" ;;
  esac
  printf '{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"%s","arguments":%s}}\n' \
    "$id" "$1" "$2" >>"$in"
}
has() { case ",$toolsets," in *",$1,"*) return 0 ;; *) return 1 ;; esac; }

if has core; then
  for tool in system_version system_status system_info settings_get whoami environment_list; do
    call "$tool" '{}'
  done
  call environment_inspect "{\"id\":$env_id}"
fi
has docker && call container_list "{\"environment\":$env_id}" probe
has swarm && call node_list "{\"environment\":$env_id}" probe
has k8s && call kubernetes_namespaces_list "{\"id\":$env_id}" probe
has helm && call helm_list "{\"id\":$env_id}" probe
has stacks && call stack_list '{}' probe
has gitops && call gitops_sources_list '{}' probe
has edge && call edge_group_list '{}' probe
has admin && call user_list '{}' probe
has observability && call status_inspect '{}' probe

# A streaming tool never returns on its own, so it is probed only when asked
# for; then the refusal itself is what gets checked. Without SMOKE_STREAM the
# list must stay free of them, or the run would wait for the client timeout.
if [ "${SMOKE_STREAM:-0}" = 1 ]; then
  has observability && call system_events "{\"environment\":$env_id}" stream
else
  if grep -qE '"name":"(system_events|container_logs|service_logs|task_logs|container_attach|exec_start)"' "$in"; then
    echo "FAIL: a streaming tool is in the probe list without SMOKE_STREAM=1" >&2
    exit 1
  fi
fi

"$tmp/portainer-mcp" --config "$config" --toolsets "$toolsets" \
  >"$tmp/out.jsonl" < <(cat "$in"; sleep 8)

reply() { grep -F "\"id\":$1," "$tmp/out.jsonl" || true; }

replies="$(grep -c '"jsonrpc"' "$tmp/out.jsonl" || true)"
tools="$(grep -o '"name":"[a-z_]*"' "$tmp/out.jsonl" | sort -u | wc -l)"
echo "toolsets: $toolsets"
echo "replies: $replies (want $id, ids 1..$id)"
echo "tool names seen: $tools"

fail=0
for core in $cores; do
  line="$(reply "$core")"
  if [ -z "$line" ] || printf '%s' "$line" | grep -q '"isError":true'; then
    echo "FAIL: core call $core: $(printf '%s' "$line" | cut -c1-240)" >&2
    fail=1
  fi
done
for probe in $probes; do
  pid="${probe%%:*}"
  pname="${probe#*:}"
  line="$(reply "$pid")"
  case "$line" in
    *'"isError":true'*)
      # A 4xx is our fault (wrong path, wrong parameter); 5xx and 503 mean the
      # environment cannot serve that feature, which the probe reports, not fails.
      if printf '%s' "$line" | grep -q 'portainer: 4'; then
        echo "FAIL: $pname answered a client error: $(printf '%s' "$line" | cut -c1-240)" >&2
        fail=1
      else
        echo "warn: $pname: environment cannot serve it: $(printf '%s' "$line" | grep -o 'portainer: [^\\]*' | cut -c1-160)"
      fi
      ;;
  esac
done
for stream in $streams; do
  sid="${stream%%:*}"
  sname="${stream#*:}"
  line="$(reply "$sid")"
  if [ -z "$line" ]; then
    echo "FAIL: $sname did not answer: the call hung instead of refusing" >&2
    fail=1
  elif ! printf '%s' "$line" | grep -q '"isError":true'; then
    echo "FAIL: $sname answered instead of refusing the unbounded stream: $(printf '%s' "$line" | cut -c1-240)" >&2
    fail=1
  elif printf '%s' "$line" | grep -q 'streams'; then
    echo "ok: $sname refused the unbounded stream"
  else
    echo "FAIL: $sname failed for another reason: $(printf '%s' "$line" | cut -c1-240)" >&2
    fail=1
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
echo "OK: core tools answered and every enabled set answered one read probe"
