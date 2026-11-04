# Surfdude for Python

A hypermedia client library that drives an API by relation names (`rel`s) instead of hardcoded URLs. You register a **dialect** that matches the server's **media type** (`SimpleDialect`, `AgentTraderDialect`). It shares the same [fixtures](../fixtures/) and neutral model as the Go and C# clients.

Installing this package provides the `surfdude` Python library for code you write. It does not install a command-line tool. If you want a command-line tool to inspect or crawl an API from your terminal, install the Go executable named **`surf`** (see [../go/README.md](../go/README.md)).

## Installation

Install the package into your Python environment:

```sh
pip install surfdude==0.4.0
```

If you use `uv`, add the package to your project:

```sh
uv add surfdude==0.4.0
```

## Toolchain (if developing in this repo)

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

## Signing in again

Give the surfer a function that gets a session and a journey never has to mention tokens:

```python
def sign_in(from_page: Page) -> str:
    # from_page is the page the server sent with the 401. If the API offers a sign-in control it is
    # right there.
    next_page = from_page.submit("sign-in", {"credential": credential})
    got: dict = {}
    next_page.read(got)
    return got["token"]

surfer = Surfer.new(url, AgentTraderDialect(), sign_in=sign_in)
```

The function runs when a step is refused with a 401: it signs in and sends the same request again, once. A surfer that holds no token yet is no different, so the first 401 is what signs it in. A control that says it `requires` a session is sent like any other, because nothing in the surfer reads `requires`. Steps that fail together on different threads share one sign-in. A step with `expect=lambda code: code == 401` is testing the refusal, so it is left alone, and the sign-in step itself can never call the function. A function that raises, or returns no token, fails the step with `SignInFailedError`. Without `sign_in` nothing changes: the server's 401 comes back as the usual `UnexpectedStatusError`.

## Publishing

PyPI package **`surfdude`**: tag **`vX.Y.Z`** on the repo triggers [Release workflow](../.github/workflows/release.yml) (trusted publisher OIDC). Install: `pip install surfdude==X.Y.Z`. Full process: [../docs/releasing.md](../docs/releasing.md).
