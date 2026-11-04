#!/usr/bin/env bash
# Bump the repo-wide semver in VERSION, Python, and .NET. Usage: ./scripts/tag-release/bump-version.sh 0.2.0
set -euo pipefail
if [ $# -ne 1 ]; then
  echo "Usage: $0 X.Y.Z" >&2
  exit 1
fi
new="$1"
if ! [[ "$new" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Expected semver X.Y.Z, got $new" >&2
  exit 1
fi

root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root"

printf '%s\n' "$new" > VERSION

sed -i.bak "s/^version = \".*\"/version = \"$new\"/" python/pyproject.toml && rm python/pyproject.toml.bak

# -i.bak then rm works on both GNU and macOS sed
sed -i.bak -e "s|<Version>.*</Version>|<Version>$new</Version>|" -e "s|<PackageVersion>.*</PackageVersion>|<PackageVersion>$new</PackageVersion>|" csharp/Directory.Build.props && rm csharp/Directory.Build.props.bak

echo "Bumped to $new. Run ./scripts/tag-release/check.sh then:"
echo "  git commit -am \"Release v$new\""
echo "  git tag v$new"
echo "  git push origin master && git push origin v$new"
