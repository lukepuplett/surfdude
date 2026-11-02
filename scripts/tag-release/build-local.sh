#!/usr/bin/env bash
# Build release artifacts locally without publishing (same outputs CI uploads).
set -euo pipefail
root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root"
v="$(tr -d '[:space:]' < VERSION)"

mkdir -p /tmp/surfdude-release
dotnet pack csharp/Evoq.Surfdude.sln -c Release \
  -p:Version="$v" -p:PackageVersion="$v" \
  -o /tmp/surfdude-release/nupkg
(cd python && uv build --out-dir /tmp/surfdude-release/pypi)
(cd go && go build -o /tmp/surfdude-release/surf ./cmd/surf)

echo "Artifacts:"
ls -la /tmp/surfdude-release/nupkg /tmp/surfdude-release/pypi /tmp/surfdude-release/surf
