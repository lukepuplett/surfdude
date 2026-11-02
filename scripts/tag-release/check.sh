#!/usr/bin/env bash
# Verify VERSION matches project files and run the same tests as CI.
set -euo pipefail
root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root"

v="$(tr -d '[:space:]' < VERSION)"
echo "VERSION=$v"

grep -q "^version = \"$v\"" python/pyproject.toml
grep -q "<Version>$v</Version>" csharp/Directory.Build.props

(cd go && go test ./...)
(cd python && uv sync --all-groups && uv run ruff check . && uv run pytest)
(cd csharp && dotnet test Evoq.Surfdude.sln -c Release)

echo "OK — ready to tag v${v}"
