# Releasing Surfdude

One **semver** in [`VERSION`](../VERSION) for Go and Python. The **.NET packages have their own version** (see below). One **git tag** `vX.Y.Z` triggers [.github/workflows/release.yml](../.github/workflows/release.yml), which publishes **Go** (module tag), **Python** (PyPI), and **`surf`** binaries (GitHub Releases). **NuGet is not automated** in that workflow; build and upload `.nupkg` yourself when ready.

## What ships where

| Output | How | Credential |
|--------|-----|------------|
| **Go module** `github.com/lukepuplett/surfdude/go` | Tag **`go/vX.Y.Z`** on the same commit as **`vX.Y.Z`** | Public git tags on this repo |
| **Python** [`surfdude`](https://pypi.org/project/surfdude/) on PyPI | `uv publish` in Release workflow | **Trusted publisher** (GitHub OIDC) — no PyPI API token in secrets |
| **`surf` CLI** (linux / darwin / windows) | [GoReleaser](../.goreleaser.yaml) → GitHub Release for **`vX.Y.Z`** | `GITHUB_TOKEN` in Actions (automatic) |
| **.NET** packages `Surfdude`, `Surfdude.SimpleJson.Client`, `Surfdude.AgentTrader.Client` | Manual (or local `dotnet nuget push`) | [nuget.org](https://www.nuget.org) API key on your machine |

The GitHub repo was renamed from `surfdude-csharp` to **`surfdude`** on 2026-11-02 (old URLs redirect). The Go module path in [`go/go.mod`](../go/go.mod) uses **`surfdude`** and now resolves.

---

## Version source of truth

| File | Purpose |
|------|---------|
| [`VERSION`](../VERSION) | Single line `X.Y.Z` |
| [`python/pyproject.toml`](../python/pyproject.toml) | `version = "X.Y.Z"` |
| [`csharp/Directory.Build.props`](../csharp/Directory.Build.props) | The **NuGet** version, `Version` / `PackageVersion`. **Separate from `VERSION`**: the packages were already on 1.x on nuget.org when Go and Python were on 0.x, and NuGet would keep offering the higher number as latest |

Align with:

```sh
./scripts/tag-release/bump-version.sh 0.2.0          # Go and Python
./scripts/tag-release/bump-csharp-version.sh 1.1.0   # NuGet packages
./scripts/tag-release/check.sh
```

`check.sh` runs Go, Python, and .NET tests, checks that `python/pyproject.toml` matches `VERSION`, and checks that the two .NET version fields agree.

Local artifacts (no publish):

```sh
./scripts/tag-release/build-local.sh
```

Outputs under `/tmp/surfdude-release/` (`.nupkg`, wheels, local `surf` binary).

---

## CI vs release

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| [.github/workflows/ci.yml](../.github/workflows/ci.yml) | Push / PR to **`master`** | Tests + version consistency (`VERSION` and Python agree; the .NET fields agree) |
| [.github/workflows/release.yml](../.github/workflows/release.yml) | Push tag **`v*.*.*`** | Tests again, then publish Go / PyPI / GitHub Release |
| [`azure-pipelines.yaml`](../azure-pipelines.yaml) | Azure DevOps | **CI only** (build + test). No NuGet push on every build. |

---

## Release flow (normal)

1. Merge to **`master`** until [CI](../.github/workflows/ci.yml) is green.
2. Bump when ready: `./scripts/tag-release/bump-version.sh X.Y.Z` and `./scripts/tag-release/check.sh`.
3. Commit, tag, push:

   ```sh
   git commit -am "Release v0.2.0"
   git tag v0.2.0
   git push origin master
   git push origin v0.2.0
   ```

4. Open **Actions → Release** for the tag. When it succeeds:
   - **`go/v0.2.0`** exists (unless it was already pushed — workflow skips duplicates).
   - **PyPI** has `surfdude==0.2.0` (if trusted publisher is configured — see below).
   - **GitHub Releases** has **`v0.2.0`** with **`surf`** assets.

### Tag pins the workflow file

GitHub runs **the `release.yml` (and permissions) from the commit the tag points at**, not from latest `master`.

If you tag an older commit, you get that commit’s release steps (for example an older PyPI step that skipped upload when no API token was set). **Merge release workflow changes to `master` first**, then create the **`vX.Y.Z`** tag on that commit (or bump patch and tag again). Re-running an old workflow run does not pick up newer workflow files on `master`.

---

## PyPI — trusted publisher (recommended)

No **`PYPI_API_TOKEN`** secret is required when OIDC is set up.

### One-time on PyPI

Account → **Trusted Publisher Management** → **Add a new pending publisher** (project **`surfdude`** may not exist until the first upload):

| Field | Value |
|--------|--------|
| PyPI project name | `surfdude` |
| Owner | `lukepuplett` |
| Repository name | `surfdude` |
| Workflow name | `release.yml` (filename under `.github/workflows/`, not the workflow display name) |
| Environment name | `pypi` (optional on PyPI; use **`pypi`** on GitHub — see below) |

After the first successful publish, the pending publisher becomes a normal project publisher.

### One-time on GitHub

Repo **Settings → Environments → New environment** → name **`pypi`**.

The Release **publish** job uses `environment: pypi`. The workflow sets `permissions: id-token: write` so **`uv publish`** can authenticate via OIDC.

### Manual PyPI (fallback)

From `python/` after `uv build`:

```sh
UV_PUBLISH_TOKEN='pypi-…' uv publish
```

Or upload wheels from `./scripts/tag-release/build-local.sh` via the PyPI web UI.

---

## Go module tag `go/vX.Y.Z`

Consumers:

```sh
go get github.com/lukepuplett/surfdude/go/surf@v0.2.0
go install github.com/lukepuplett/surfdude/go/cmd/surf@v0.2.0
```

The Release workflow creates **`go/vX.Y.Z`** pointing at the tagged commit.

### When Actions cannot push `go/v…`

If the **release commit changes files under `.github/workflows/`**, GitHub’s default **`GITHUB_TOKEN`** may refuse to push the module tag (same class of error as pushing workflow changes from a local OAuth token without **`workflow`** scope).

**Fixes:**

1. **Optional secret `GH_RELEASE_PAT`:** fine-grained or classic PAT with **Contents** and **Workflows** write. Checkout uses it for the publish job so `git push origin go/v…` succeeds.
2. **Push the module tag locally** (your `gh`/git already has **`workflow`** scope if you fixed Actions pushes from your machine):

   ```sh
   git fetch origin tag v0.2.0
   git tag go/v0.2.0 v0.2.0
   git push origin go/v0.2.0
   ```

If **`go/v…`** already exists on the remote, the workflow skips creating it.

### Pushing workflow files from your machine

If `git push` rejects updates to `.github/workflows/*`, refresh GitHub CLI auth:

```sh
gh auth refresh -h github.com -s workflow
```

Git credential helper should use **`gh auth git-credential`** for `github.com`.

---

## GitHub Releases (`surf` binary)

[`.goreleaser.yaml`](../.goreleaser.yaml) builds [`go/cmd/surf`](../go/cmd/surf) for common OS/arch pairs. GoReleaser attaches assets to the Release for tag **`vX.Y.Z`**.

Download from [GitHub Releases](https://github.com/lukepuplett/surfdude/releases) or use **`go install`** (see above).

---

## Release notes

[CHANGELOG.md](../CHANGELOG.md) holds the notes for every release. Add the new version's section before you bump and tag, and paste it into the GitHub release.

## .NET / NuGet (manual)

The Release workflow **does not** pack or push NuGet packages. Reasons in practice: separate Microsoft account / API key flow; packages can ship on a different cadence than Go/Python. They also have their own version number: nuget.org already had `Surfdude` and `Surfdude.SimpleJson.Client` at 1.0.3, so a 0.x upload would be listed as older and NuGet would keep offering 1.0.3. Uploads cannot be deleted, only unlisted, so check the number before you push.

Build packages locally:

```sh
./scripts/tag-release/build-local.sh
# or
dotnet pack csharp/Evoq.Surfdude.sln -c Release -o ./artifacts/nupkg
```

Upload **`Surfdude.*.nupkg`**, **`Surfdude.SimpleJson.Client.*.nupkg`** and **`Surfdude.AgentTrader.Client.*.nupkg`** (not the console app unless you intend to publish it):

```sh
dotnet nuget push ./artifacts/nupkg/Surfdude.1.1.0.nupkg \
  --api-key YOUR_KEY --source https://api.nuget.org/v3/index.json
```

To automate NuGet later, add a pack/push step back to `release.yml` and secret **`NUGET_API_KEY`**.

---

## What consumers run

Replace **`0.1.0`** with the Go or Python version you shipped (the .NET row uses its own version).

| Ecosystem | Library | CLI |
|-----------|---------|-----|
| **Go** | `go get github.com/lukepuplett/surfdude/go/surf@v0.1.0` | `go install github.com/lukepuplett/surfdude/go/cmd/surf@v0.1.0` or GitHub Release **`surf`** |
| **Python** | `uv add surfdude==0.1.0` or `pip install surfdude==0.1.0` | Go **`surf`** or Release binary (no PyPI CLI yet) |
| **.NET** | `dotnet add package Surfdude --version 1.1.0`, `Surfdude.SimpleJson.Client` and `Surfdude.AgentTrader.Client` (use the .NET version, not `VERSION`) | No global tool yet |

---

## First-time checklist

- [ ] PyPI **pending trusted publisher** for **`surfdude`** → `lukepuplett/surfdude`, workflow **`release.yml`**, environment **`pypi`** (or “Any” on PyPI if you omit GitHub environment — not recommended).
- [ ] GitHub environment **`pypi`** created.
- [ ] Optional **`GH_RELEASE_PAT`** if module tag push fails from Actions.
- [ ] `./scripts/tag-release/check.sh` green on **`master`**.
- [ ] Push **`vX.Y.Z`** on a commit that includes the current **`release.yml`** (OIDC PyPI step).
- [ ] Smoke-test Go get, PyPI install, and Release **`surf`** binary.
- [ ] NuGet: when ready, manual upload from **`build-local.sh`** output.

---

## Troubleshooting

| Symptom | Likely cause |
|---------|----------------|
| Release green but **no PyPI project** | Tag pointed at an **old** commit whose workflow skipped PyPI (no token / pre-OIDC). Tag again from current **`master`** or publish manually. |
| **`go/v…` push failed** in Actions | Release commit touched **workflows**; add **`GH_RELEASE_PAT`** or push **`go/v…`** locally. |
| **`git push` rejected** for workflow files locally | Run **`gh auth refresh -h github.com -s workflow`**. |
| GoReleaser failed | Check Actions log; tag must match **`VERSION`** and tests must pass in **verify** job. |

---

## Internal (Pacific AM)

For private feeds, point **`uv publish`** and **`dotnet nuget push`** at Azure Artifacts (or similar) in a fork of the release workflow; build steps stay the same.
