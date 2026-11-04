#!/usr/bin/env bash
# Bump the NuGet packages' own semver in csharp/Directory.Build.props. Usage: ./scripts/tag-release/bump-csharp-version.sh 1.1.0
# The C# packages were already on 1.x when Go and Python were on 0.x, so they do not follow /VERSION.
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

# -i.bak then rm works on both GNU and macOS sed
sed -i.bak -e "s|<Version>.*</Version>|<Version>$new</Version>|" -e "s|<PackageVersion>.*</PackageVersion>|<PackageVersion>$new</PackageVersion>|" csharp/Directory.Build.props && rm csharp/Directory.Build.props.bak

echo "NuGet packages bumped to $new. Run ./scripts/tag-release/check.sh, add the section to CHANGELOG.md, then pack and upload (docs/releasing.md)."
