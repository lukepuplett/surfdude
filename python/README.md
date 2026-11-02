# Surfdude for Python

Hypermedia client: drive an API by `rel` names, not URLs. Same neutral model and shared [fixtures](../fixtures/) as Go and C#. [Form values and prefills](../docs/model.md#form-values-for-api-writers-and-for-journeys) · [C# client](../csharp/README.md)

## Toolchain (if you are new to Python here)

| Piece | What we use | Why |
|-------|-------------|-----|
| **Python version** | **3.12+** (`requires-python` in `pyproject.toml`) | Modern typing and long support window |
| **Local pin** | `.python-version` | Tells [uv](https://docs.astral.sh/uv/) which interpreter to install for this folder |
| **Package manager** | **uv** only | Creates `.venv`, installs deps, runs pytest — no system `pip` needed |
| **HTTP** | **httpx** | Sync client (async can come later) |
| **Tests** | **pytest** | Conformance runs every row in `../fixtures/*/cases.json` |
| **Lint** | **ruff** | Style and import hygiene |

You do not need a global Python install if uv is on your PATH: uv downloads the pinned version on first `sync`.

## Commands

```sh
cd python
uv sync --all-groups          # create .venv and install surfdude + dev tools
uv run pytest                 # conformance + wave journey tests
uv run ruff check src tests   # lint
```

## Layout

```text
src/surfdude/
  hypertext/     Resource, Control, errors
  dialect/       registry and HTTPResponse for dialects
  dialects/      simple, agenttrader
  surf/          Surfer, Page, Wave
  conformance.py shared fixture runner (same cases as Go)
tests/
```

## Sample ride

```python
from surfdude import AgentTraderDialect, Surfer

with Surfer.new("http://localhost:8080", AgentTraderDialect()) as s:
    s.wave().from_root().then_submit(
        "search-listings", {"make": "MINI"}
    ).then_item(0).then_submit(
        "shortlist", {"note": "nice colour"}
    ).go()
```

See [../docs/model.md](../docs/model.md) and [../docs/dialects.md](../docs/dialects.md).
