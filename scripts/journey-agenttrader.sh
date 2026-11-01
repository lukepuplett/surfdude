#!/usr/bin/env bash
# Starts a local AgentTrader, then drives it with surf: show, explore, and the example journeys.
# Needs Go, and a checkout of agenttrader (default ../agenttrader, or set AGENTTRADER_DIR).
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
agenttrader_dir="${AGENTTRADER_DIR:-$here/../agenttrader}"
port="${PORT:-18081}"
tmp="$(mktemp -d)"
trap 'kill "${pid:-0}" 2>/dev/null || true; rm -rf "$tmp"' EXIT

(cd "$agenttrader_dir" && go build -o "$tmp/agenttrader" ./cmd/agenttrader)
PORT="$port" "$tmp/agenttrader" >"$tmp/server.log" 2>&1 &
pid=$!

for _ in $(seq 1 20); do
  if curl -fs "http://localhost:$port/health" >/dev/null 2>&1; then break; fi
  sleep 0.5
done

url="http://localhost:$port/"
rels="$agenttrader_dir/doc/rels.md"

cd "$here/go"
go build -o "$tmp/surf" ./cmd/surf
echo "== surf show"
"$tmp/surf" show "$url"
echo
echo "== surf explore"
"$tmp/surf" ${rels:+-known-rels "$rels"} explore "$url"
echo
echo "== journeys"
AGENTTRADER_URL="$url" go test -v ./examples/agenttrader
