# Releasing Surfdude

One **semver** for Go, Python, and .NET, one **git tag** `vX.Y.Z`, one **GitHub Actions** workflow that publishes everywhere. You create registry accounts once, store **API tokens as GitHub secrets**, and never upload through a web UI unless you want to.

## Accounts (one-time)

| Registry | Sign up | Credential | GitHub secret |
|----------|---------|------------|---------------|
| [nuget.org](https://www.nuget.org) | Organisation or user | Profile → **API keys** → create | `NUGET_API_KEY` |
| [pypi.org](https://pypi.org) | User + **2FA** recommended | Account → **API tokens** → scope to project `surfdude` | `PYPI_API_TOKEN` |
| **Go** | *(none)* | Public git tags on this repo | — |
| **GitHub Releases** (`surf` binary) | Same repo | `GITHUB_TOKEN` (automatic in Actions) | — |

Go modules use tags `go/vX.Y.Z` on the same commit as `vX.Y.Z`. The release workflow creates `go/v…` after you push `v…`.

Optional: run `./scripts/tag-release/build-local.sh` and upload `.nupkg` / wheels manually with `dotnet nuget push` and `uv publish` if CI is not wired yet.

## Version source of truth

| File | Purpose |
|------|---------|
| [`VERSION`](../VERSION) | Single line `X.Y.Z` |
| [`python/pyproject.toml`](../python/pyproject.toml) | `version = "X.Y.Z"` |
| [`csharp/Directory.Build.props`](../csharp/Directory.Build.props) | `Version` / `PackageVersion` |

Keep them aligned:

```sh
./scripts/tag-release/bump-version.sh 0.2.0
./scripts/tag-release/check.sh
```

## Release flow (normal)

1. Merge work to **`master`** (CI runs [.github/workflows/ci.yml](../.github/workflows/ci.yml): Go, Python, .NET, version check).
2. Bump version when ready for a release (script above).
3. Commit, tag, push:

   ```sh
   git commit -am "Release v0.2.0"
   git tag v0.2.0
   git push origin master
   git push origin v0.2.0
   ```

4. [.github/workflows/release.yml](../.github/workflows/release.yml) runs on the tag:
   - Confirms tag matches `VERSION`
   - Re-runs tests
   - Tags **`go/v0.2.0`** for `go get github.com/lukepuplett/surfdude/go@v0.2.0`
   - **`dotnet pack`** → NuGet.org (if `NUGET_API_KEY` set; otherwise packs only, logs skip)
   - **`uv build` / `uv publish`** → PyPI (if `PYPI_API_TOKEN` set)
   - **GoReleaser** → GitHub Release with **`surf`** binaries (linux/darwin/windows)

If secrets are missing, the workflow still passes but skips upload and prints where local artifacts would go—useful for dry runs.

## What consumers run after `v0.1.0`

Replace `0.1.0` with the tag you shipped.

| Ecosystem | Library | CLI |
|-----------|---------|-----|
| **Go** | `go get github.com/lukepuplett/surfdude/go/surf@v0.1.0` | `go install github.com/lukepuplett/surfdude/go/cmd/surf@v0.1.0` |
| **Python** | `uv add surfdude==0.1.0` or `pip install surfdude==0.1.0` | *(library only today; CLI via `go install` or GitHub Release binary)* |
| **.NET** | `dotnet add package Surfdude --version 0.1.0` and `Surfdude.SimpleJson.Client` | *(no global tool yet)* |

Module path still says `surfdude-csharp` on GitHub until the repo rename; `go get` uses the module path in [`go/go.mod`](../go/go.mod).

## Azure Pipelines

[`azure-pipelines.yaml`](../azure-pipelines.yaml) is **CI only** (build + test). It does **not** push to NuGet on every build. Use GitHub tag releases for publishing, or add a separate manual pipeline gated on tags if PAM standardises on Azure Artifacts later.

## First-time checklist

- [ ] Create NuGet.org account; register package ids **Surfdude** / **Surfdude.SimpleJson.Client** (or verify you own them).
- [ ] Create PyPI account; reserve name **`surfdude`** (upload first release claims it).
- [ ] In GitHub repo **Settings → Secrets and variables → Actions**, add `NUGET_API_KEY` and `PYPI_API_TOKEN`.
- [ ] Run `./scripts/tag-release/check.sh` on `master`.
- [ ] Push tag `v0.1.0` (or bump first), confirm Release workflow green.
- [ ] Smoke-test install commands from the table above.

## Internal (Pacific AM)

For private feeds, replace NuGet/PyPI URLs with **Azure Artifacts** (or similar) in the release workflow and use the same `.nupkg` / wheel artifacts—no change to how you build them.
