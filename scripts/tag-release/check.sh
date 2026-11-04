#!/usr/bin/env bash
# Verify VERSION matches the Python project, the C# versions agree, and run the same tests as CI.
set -euo pipefail
root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root"

v="$(tr -d '[:space:]' < VERSION)"
echo "VERSION=$v"

grep -q "^version = \"$v\"" python/pyproject.toml
cv="$(sed -n 's|.*<Version>\(.*\)</Version>.*|\1|p' csharp/Directory.Build.props)"
grep -q "<PackageVersion>$cv</PackageVersion>" csharp/Directory.Build.props
echo "NuGet version=$cv"

(cd go && go test ./...)
(cd python && uv sync --all-groups && uv run ruff check . && uv run pytest)
(cd csharp && dotnet test Evoq.Surfdude.sln -c Release)

echo "OK — ready to tag v${v} (NuGet packages are $cv)"
