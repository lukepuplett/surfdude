# AGENTS.md — Surfdude (`surfdude-csharp` repo)

Hypermedia API client in three languages (Go, Python, C#) plus shared **fixtures** and **docs**. Journeys follow **`rel`** names, not URLs; **dialects** are pluggable **media type** readers.

**Read first:** root [README.md](README.md) (especially [Dialect = media type](README.md#dialect--media-type)), [docs/model.md](docs/model.md), [docs/dialects.md](docs/dialects.md).

---

## Vocabulary

| Say | Means |
|-----|--------|
| **Dialect** | Client plugin: `Accept`, `Matches`, `Read` for one hypermedia spelling (= **media type** in HTTP/API design). |
| **simple** | `application/simple-hypertext+json` — C# package `Evoq.Surfdude.SimpleJson`. |
| **agenttrader** | AgentTrader JSON envelope + profile — Go/Python only today. |
| **Wave / journey** | Fluent chain of steps (`FromRoot`, `Then`, `ThenSubmit`, `Go`). |
| **Fixture** | Shared JSON under `fixtures/<dialect>/` + `cases.json` conformance. |

C# maps **dialect → `IHypertextResourceFormatter`** (not the word “dialect” in API names).

---

## Repo layout

| Path | Stack | Notes |
|------|--------|--------|
| [go/](go/) | Go 1.21+ module `github.com/lukepuplett/surfdude/go` | `surf`, `dialects/`, `cmd/surf`, [go/README.md](go/README.md) |
| [python/](python/) | uv, pytest, httpx | [python/README.md](python/README.md) — **uv only** for Python |
| [csharp/](csharp/) | net8.0 / net10.0 | [csharp/README.md](csharp/README.md), tests in `csharp/tests/Evoq.Surfdude.Tests` |
| [docs/](docs/) | Language-neutral model and dialect contract | |
| [fixtures/](fixtures/) | Conformance truth per dialect (media type) | |
| [tmp/gap-tracking.md](tmp/gap-tracking.md) | Behaviour gaps and fix status | not for end-user docs |

---

## Commands (verify before claiming green)

```sh
cd go && go test ./...
cd python && uv sync --all-groups && uv run pytest && uv run ruff check .
cd csharp && dotnet test Evoq.Surfdude.sln
```

Go live journeys may skip if AgentTrader is unreachable from the environment.

**Releasing:** [docs/releasing.md](docs/releasing.md) — bump `VERSION`, tag `vX.Y.Z`, GitHub Actions publishes NuGet/PyPI/GoReleaser when secrets are set. Scripts under `scripts/tag-release/`.

---

## Implementation rules

- **Neutral model first:** hypermedia differences stay in **dialect** / formatter, not in wave logic.
- **Shared behaviour:** add or extend [fixtures](fixtures/) and conformance before (or with) code; Go `internal/conformance`, Python `surfdude.conformance`, C# tests as needed.
- **Prefills:** server **`value`** on inputs merges with journey submit maps (all three clients).
- **Retries:** default none; Go/Python opt-in policy; C# via `HttpClient` handlers (Polly / resilience).
- **Do not** commit secrets. **Do not** commit unless the user asked (this repo often lands on **`master`** directly when they do).

---

## Docs

- User-facing: [README.md](README.md), [docs/](docs/), language READMEs.
- **Dialect = media type** must stay aligned across README and [docs/dialects.md](docs/dialects.md).
- Team wiki (Pacific AM): clone [Wiki.wiki](https://github.com/lukepuplett/Wiki.wiki) separately; durable cross-repo process lives there, not duplicated here unless asked.

---

## Git

- Default branch: **`master`**. User frequently requests push to `origin master` without PR.
- Remote name may still be `surfdude-csharp`; rename to `surfdude` is a known roadmap item (Go module path already assumes `surfdude`).

---

## Related

- [Agent instruction files (PAM wiki)](https://dev.azure.com/pacificam/Wiki/_wiki/wikis/Wiki.wiki?pagePath=/Coding/Tooling/Agent-Instruction-Files-AGENTS-md-CLAUDE-md) — how to write AGENTS.md in other repos.
