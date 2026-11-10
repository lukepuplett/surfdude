# Surfdude behaviour gaps — tracking

All ten gaps found in review are now fixed in both the Go and Python clients. The repro tests that
proved each gap are now ordinary regression tests and run with the default suites:

```sh
cd go && go test ./...
cd python && uv run pytest
```

Last updated: 2026-11-02

## Fixed

| ID | Gap | What now happens | Go | Python |
|----|-----|------------------|----|--------|
| GAP-01 | `Requires` on journeys | **Superseded in v0.3.0.** `requires` is an extension and the surfer does not read it. A control that says it requires a session is sent like any other, and the server's 401 decides (with `WithSignIn`, that 401 signs in). | `TestGAP01_RequiresIsNotActed…` | `test_gap01_requires_is_not_acted…` |
| GAP-02 | `Price` on controls | **Superseded in v0.3.0.** `price` is an extension and the surfer does not read it. A priced control is sent, and the server's 402 comes back as `UnexpectedStatusError`. A 402 handler (not built) is where payment belongs. | `TestGAP02_PriceIsNotActed…` | `test_gap02_price_is_not_acted…` |
| GAP-03 | `Control.Type` non-hypermedia | **Superseded in v0.3.0.** `type` is an extension. Following such a control sends the request, and an answer no dialect can read is reported as `UnsupportedMediaTypeError`. `Explore` still skips these. | `TestGAP03_TypeIsNotActed…` | `test_gap03_type_is_not_acted…` |
| GAP-04 | `Page.Read` on a list | Reading a page that has several items fails and tells the caller to select one with `Item(i)`. | `MultiItemReadError` | `MultiItemReadError` |
| GAP-05 | Read after a refusal | When a resource carries problems (for example a 422), `Read` returns `NoDataError` instead of decoding problem rows as business data. | `Resource.Decode` | `Resource.decode` |
| GAP-06 | Wave from an arbitrary URL | A wave can start at a bookmarked path or URL. | `Wave.From`, `Surfer.Open` | `Wave.from_url`, `Surfer.open` |
| GAP-07 | Raw body on a page | The whole response body is available for decoding outside the neutral model. | `Page.RawBody` | `Page.raw_body` |
| GAP-08 | Wave with an expected status | A wave step can expect a refusal such as 422 and carry on. Go already supported this through `surf.Expect`. | already supported | `then(..., expect=)`, `then_submit(..., expect=)` |
| GAP-09 | JSON Schema `format` | A value that does not match `format: email` is rejected before sending. Other formats are not yet checked. | `checkFormat` in `surf/values.go` | `_check_format` in `surf/values.py` |
| GAP-10 | Duplicate `rel` | A resource can report which rels appear on more than one control. This is reporting only; it does not yet change which control wins. | `Resource.DuplicateRels` | `Resource.duplicate_rels` |
| GAP-11 | A response no dialect can read | The page that comes back with the error can be asked for rels, notes, items and the body without a panic (Go dereferenced a nil `Resource`), and `UnexpectedStatusError` carries `Header` and `Body`, so a test can read `Retry-After` or `WWW-Authenticate` or see a proxy's plain-text page from the error alone. | `finish` in `surf/surf.go`, `UnexpectedStatusError.Header` and `.Body` | not checked yet |
| GAP-12 | Sign in again | A surfer can be given `WithSignIn(func)`. It signs in and replays the request once on a 401 (with no token held, the first 401 is what signs in). Concurrent steps share one sign-in; an expected 401 and the sign-in step itself never trigger it. | `WithSignIn`, `signInAgain` in `surf/surf.go` | Python: `Surfer.new(sign_in=)`, `_sign_in_again` in `surf/surfer.py`. C#: `SignInHandler` in the `HttpClient` pipeline |
| GAP-13 | Record samples from the real response | `surftest.Golden` checks a response against a checked-in JSON file, replacing the server's address with `{base}`, masking volatile fields, keeping the server's key order, and writing the file when `SURFDUDE_UPDATE_GOLDEN` is set. A mismatch lists the JSON paths that differ. | `surftest.Golden`, `surftest.Mask` | not built |

## Behaviour to know about

- There is no preflight. The surfer sends a control that says it requires a session or has a price, and
  the server's 401 or 402 decides. A 402 handler and the x402 dialect do not exist yet.
- `format` validation covers `email` only. Unknown formats are ignored rather than rejected, which
  matches the JSON Schema rule that unknown formats are annotations.
- A duplicate rel is reported but not rejected. The first control with that rel still wins.

## Real-client hardening (done 2026-11-02)

Built test first in each language. The opt-in retry rules are in [docs/model.md](../docs/model.md#retries-timeouts-and-credentials).

| Item | Go | Python | C# |
|------|----|--------|----|
| Opt-in, step-aware retry policy, with a standard preset | `surf.WithRetry`, `surf.Retrying`, `surf.StandardRetry`, `surf.Never` | `Surfer.new(retry=)`, `retry=` on steps, `StandardRetry`, `never` | Not built in. Put a handler in the `HttpClient`; a test shows a retrying handler works across waves. |
| Prefilled **`value`** on simple `form[]` and AgentTrader `fields[]` | merge on submit | merge on submit | merge in `BuildRequest` |
| POST retried only with an idempotency key | `surf.WithIdempotencyKey` | `idempotency_key=True` | Left to the handler | `Page.Attempts`, `ReportLine.Attempts` | `Page.attempts`, `ReportLine.attempts` | Not applicable |
| The failing step shows on the report | Fixed in `Wave.Go` | `WaveStoppedError.report` (before, the report was lost on failure) | Already the case |
| Transport failure is a clear, wrapped error | Already so | `RequestFailedError` (before, a raw `httpx` error escaped) | Not changed |
| Configurable timeout | `surf.WithTimeout` | `Surfer.new(timeout=)` | Set on the `HttpClient` the caller gives |
| Bearer token changed safely while requests run | `RWMutex` in `Surfer` | Not needed, a single attribute write is atomic | Not applicable |
| Reusable client with a shared `HttpClient` | Already so | Already so | `Surfer.For(...).WithHttpClient(...)`; `StepFactory` now shares one default client |
| Journey settings do not leak between waves | Already so | Already so | `Surfer.Wave(Action<SurfContext>)` makes a fresh context per wave |

Changes a maintainer should know about:

- `Evoq.Surfdude.SimpleJson` now has a `ProjectReference` to `Evoq.Surfdude`. It used to reference the published `Surfdude` 1.0.0 package, so it was built against an old copy of its own base library and could not see any change made here. Packing now makes the SimpleJson package depend on the matching `Surfdude` version.
- The C# libraries target **net8.0** and **net10.0**; the console app and tests target **net10.0**. See [csharp/README.md](../csharp/README.md); run `dotnet test csharp/tests/Evoq.Surfdude.Tests`.

## To port to Python and C# (from the Go work released as v0.2.0)

GAP-11, GAP-12 and GAP-13 were built in Go first. The Go tests are the specification: copy their scenarios, not their code.

### Python (`python/`, httpx)

| Gap | What to do | Spec to copy |
|-----|------------|--------------|
| GAP-11 | **First check whether Python has the bug.** Open a plain-text 404 and ask the page that comes back with the error for `rels`, `has`, `notes`, `item`, `read`, `follow`. None may raise `AttributeError` on a missing resource. The page must keep status, headers and raw body when no dialect can read the response, and the unexpected-status error must carry `headers` and `body`. | `go/surf/unreadable_test.go` |
| GAP-12 | **Built** as `sign_in=fn` on `Surfer.new`; tests in `tests/test_sign_in.py`. Original brief: add `with_sign_in(fn)` to the surfer. `fn(from_page)` returns the token. It runs on a 401 (replay the request once, body intact, new token). Guards: a step that expects 401 is left alone; the sign-in step cannot re-enter the hook (use a `contextvars.ContextVar`); one replay per step; concurrent steps share one sign-in (a `threading.Lock` and a changed-token check; async is separate work); a hook that raises or returns no token fails the step with the cause chained; with no hook nothing changes. | `go/surf/signin_test.go` (ten scenarios) |
| GAP-13 | Python has no `surftest` module yet (already listed below). When it gets one, add `golden(page, path, mask=...)` with `SURFDUDE_UPDATE_GOLDEN`, `{base}` for the page's origin, `<masked>` paths such as `data[0].expiresAt`, two-space indent, and a failure that lists the differing JSON paths. Dicts keep insertion order, so the server's key order is free. | `go/surftest/golden_test.go` |

### C# (`csharp/`)

| Gap | What to do | Spec to copy |
|-----|------------|--------------|
| GAP-11 | Check that a response no formatter can read gives a resource that is safe to ask (empty, not null), keeps the status, headers and raw body, and that the unexpected-status exception carries `Headers` and `Body`. | `go/surf/unreadable_test.go` |
| GAP-12 | **Built** as `SignInHandler`; tests in `SignInHandlerTests.cs`. Not built: a `Surfer.WithSignIn` (a surfer cannot add a handler to a client it was given, so the caller puts the handler in the pipeline). Original brief: belongs in the `HttpClient` pipeline, like retries: a `DelegatingHandler` that, on a 401 from a request not marked as expecting it, calls a sign-in delegate, sets the bearer and sends the request again once. Content can only be sent twice if buffered (`LoadIntoBufferAsync`) or rebuilt. Concurrent requests share one sign-in (`SemaphoreSlim` and a changed-token check). Same guards as Go. | `go/surf/signin_test.go` |
| GAP-13 | C# has no test-helper package. Consider an existing snapshot tool (Verify, Snapper) with a scrubber for the base address and volatile fields before writing one. | `go/surftest/golden_test.go` |

## To port to Python and C#: the `surf` command line that sends (Go, unreleased)

Go has a command line that can send, not only `show` and `explore`: `surf ROOT REL name=value ... [then REL ...]`, `surf run FILE`, `surf describe ROOT REL`, `--dry-run`, `--sign-in`, `--secret`, `--json`, `-q`. It is described in `docs/cli.md`. Python has no command line at all (listed below). A C# console app exists (`csharp/`); whether to give it the same commands, or to ship the Go `surf` as the one tool, is undecided.

To port, copy the Go tests in `go/cli/` as the specification, not the code. What each file pins:

| Go test file | What it specifies |
|---|---|
| `values_test.go` | the value syntax: `name=text`, `name:=JSON` (a number keeps its exact digits), `@file`, `@-` (read once), `env:VAR` (unset is an error); which values are secret (by source, or by a fixed list of full field names, see `hidden_test.go`) |
| `chain_test.go` | steps joined by `then`; `--keep NAME=PATH[~PATTERN]` (first group, else whole match); `--secret NAME`; `{name}` must be kept by an earlier step, checked before anything is sent; only `name=text` values are templates |
| `run_test.go`, `run_chain_test.go` | one step and a chain: a mistake is refused before the first request; the exit codes (0 worked, 1 refused, 2 badly formed, 3 network, 4 server, 5 rel not offered); the chain stops at the first failure; only the last response is printed; refusals go to stderr (stdout stays empty, except with `--json`) |
| `frontdoor_test.go` | a step's rel must be offered by the page it starts from, so a rel only the front door offers fails and names `--front-door`; with the flag it is taken from the front door and one note goes to stderr (never stdout); the first step never needs the flag; the sign-in lookup and a script follow the same rule |
| `notoffered_test.go` | exit 5 at every lookup (a step, `describe`, the sign-in rel, a script, the library's own rel error); exit 2 is kept for a badly formed command; `Did you mean "REL"?` only when exactly one offered rel is close (two edits or fewer, or one contains the other, case ignored; an exact match is not a typo; a rel listed twice counts once) |
| `hidden_test.go` | the hidden list is pinned (`credential token accesstoken refreshtoken password secret clientsecret apikey privatekey authorization`); a name matches when equal after lower-casing and removing `_` and `-`, a dotted name by its last part; near misses (`passwordWasRight`, `tokenizer`, `userPassword`, `apiToken`) are shown; `--reveal` shows name-hidden values in the summary, `describe` and a dry run, but never source-hidden values or the bearer token; `--json` and `-q` always show real values; a kept value is secret by its name or the last part of its path |
| `dryrun_test.go` | the request is shown and not sent; secrets and the bearer token are masked; a file value shows its source; a chain shows only the first step |
| `signin_test.go` | on a 401, follow `sign-in` from the 401 page (the front door only with `--front-door`), take the token at `--token-path`, retry once; sign-in values are read before anything is sent and cannot be templates |
| `secret_test.go` | `--secret`: one line of stdin per secret when not at a terminal, a hidden prompt at one, and a refusal rather than echo when typing cannot be hidden |
| `script_test.go` | `run FILE`: `root URL` first, one step per line, `#` comments, shell-style quotes, errors name their line |
| `describe_test.go` | fields, required stars, limits, a fill-in line |
| `legacy_test.go` | `show` and `explore` unchanged |

The one library change: Go's `Page.Prepare` (build a request without sending it, with the same checks as `Submit`). Python and C# need the equivalent for `--dry-run`.

Added after that (Go, unreleased), to port with the same tests as the specification:

| Go test file | What it specifies |
|---|---|
| `cli/expect_test.go` | `--expect 422,429` replaces "any 2xx" as the step's success; a surprise status (a 2xx too) fails with its usual code; the next rel must be on the expected page; a step expecting 401 does not sign in; bad lists are exit 2 with nothing sent |
| `cli/unchecked_test.go`, `surf/unchecked_test.go`, `surf/unchecked_step_test.go` | `--unchecked` / `Unchecked()` skip the local checks (unknown field, missing required, schema, read-only) and nothing else; one stderr note naming no value; per step; dry run still masks |
| `cli/trace_test.go` | `-v`: `step N rel METHOD STATUS Xms [kept=a,b]` on stderr; no value, ever; failing steps traced; injected clock |
| `cmd/surf/prompt_test.go` | echo restored after a line, a failed read, a panic, Ctrl-C (exit 130) and SIGTERM (143); restore before exit; once only. Port the prompt behind a terminal interface and also check by hand under a real pty (`docs/cli.md`) |
| `hypertext/fixed_test.go`, `surf/explore_collision_test.go`, `cli/fixed_test.go`, `dialects/*/readonly_test.go` | `readOnly` read into Input (agenttrader: from the schema; simple: `"readOnly": true`); a different caller value refused, the same accepted; fixed marks in `describe` and `next:`; a field repeating a literal fixed href parameter is an explorer problem and a `describe` warning (templates and values containing `{` are ignored) |
| `surftest/refusedpage_test.go` | `RefusedPage` returns the refused page |

Library semantics to port: `Unchecked` step option; `Input.ReadOnly`; `ReadOnlyInputError`; `Control.RepeatedFixedParams`; the explorer finding.

## Still open (not covered by the tests above)

- Python is synchronous only. An async client (`httpx.AsyncClient`) is a separate piece of work and needs its own retry and sleep seam.
- Go, Python and C# can sign in again on a 401 (GAP-12). Request signing and anything beyond a bearer token is still not designed; today it belongs in the HTTP client's pipeline.
- C# has no per-step retry or timeout. Both belong in the `HttpClient` pipeline, so one wave cannot opt out of the surfer's policy.
- C# does not run the shared `/fixtures` conformance cases.
- Cookie sessions: only `Authorization: Bearer` is modelled.
- Redirect policy is unspecified, with no strict mode.
- The sign-in hook returns the token and the surfer (Go, Python) or the handler (C#) keeps it.
- The x402 payment flow itself.
- Python still lacks Explore, a command-line tool, `surftest`-style helpers and live AgentTrader
  journeys.
- `Explore` (Go) and the `surf` command read the `requires` and `type` extensions to decide what to
  skip and what to show. They are tools, not the request path.

## C# AgentTrader client (experiment: extras in a dictionary)

`csharp/src/Evoq.Surfdude.AgentTrader` (`Surfdude.AgentTrader.Client`) reads the AgentTrader envelope and builds requests, with `requires` and `price` carried in `IHypertextControl.ControlData` as JSON text, not as members of the interface. Tests: `AgentTraderTests.cs`, driven by `fixtures/agenttrader/cases.json`.

What it showed:

- The existing key/value bag was enough. No core interface changed. `requires` and `price` were never needed to build a request or to sign in on a 401.
- The C# model cannot hold a resource's status, notes, paging or problems. Those parts of `cases.json` are not checked in C#. Reading a 422's problems from a C# journey is still not possible.
- Input schema is read only for `type` (so a number is sent as a number). `enum`, `minimum`, `pattern` and the other checks Go and Python make are not done in C#.
- URI templates in an href are not expanded.
- Fixed in core: `DetermineHttpMethod` lowercased the method (`post`). It now uppercases.
- `AgentTraderSurfer` and `AgentTraderSurf` copy `Surfer` and `Surf` from the SimpleJson package. A formatter-taking `Surfer` in the core would remove the copy.

## Done in v0.3.0: extras live in a bag in all three languages

The rule in `docs/model.md` ("Core parts and extras") is now built everywhere. `Requires`, `Price`, `Type` and `Title` are gone from the Go and Python `Control`; the dialects keep every key the model has no field for as an extension (`Control.Extensions` in Go, `Control.extensions` in Python, `ControlData` in C#). The preflight, `RequiresNotMetError`, `PaymentRequiredError`, `NonHypermediaControlError` and Go's `hypertext.Price` were removed. `fixtures/agenttrader/cases.json` expects `extensions.requires`, and Go, Python and C# check it.

Breaking for anyone who used those fields or errors.

## Done in v0.6.0: challenge registry (Go, Python; C# in repo)

- **Pluggable challenges** in the surf library: ordered matchers on status, headers and body; handler runs on the refused page; **one replay** of the failed request (same idea as sign-in). Go: `WithChallenges`, `Challenge`; agenttrader `MatchCapabilityGap` vs `IsX402Body` / `MatchX402Payment`; envelope `missing` on the resource. Python: `Surfer.new(..., challenges=)`, `test_challenge.py` (57 pytest pass, 2026-11-10). C#: `WithChallenges`, `ChallengeHandlerTests.cs` — **not run on every dev box**: test projects target **net10.0**; SDK 9.0 refuses restore (`NETSDK1045`). Verify with .NET 10 SDK or CI that already builds net10.
- **Still left:** x402 **payment execution** (wallet header retry), a full x402 parser/dialect, and wiring challenges into CLI send (Go CLI unchanged). `WithSignIn` remains separate from the registry; unifying 401 into one registry is optional later. C# optional: re-entry guard on challenge handlers (Go/Python have one).
