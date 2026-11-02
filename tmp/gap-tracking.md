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
| GAP-01 | `Requires` on journeys | A control that needs `agent-session` is refused before any HTTP request when the surfer holds no token. The error names the requirement. | `surf/preflight.go`, `RequiresNotMetError` | `surf/preflight.py`, `RequiresNotMetError` |
| GAP-02 | `Price` on controls | A control that carries a price is refused before the request is sent, and the error says payment is required. | `PaymentRequiredError` | `PaymentRequiredError` |
| GAP-03 | `Control.Type` non-hypermedia | A control whose type is not JSON is refused before the request is sent. | `NonHypermediaControlError` | `NonHypermediaControlError` |
| GAP-04 | `Page.Read` on a list | Reading a page that has several items fails and tells the caller to select one with `Item(i)`. | `MultiItemReadError` | `MultiItemReadError` |
| GAP-05 | Read after a refusal | When a resource carries problems (for example a 422), `Read` returns `NoDataError` instead of decoding problem rows as business data. | `Resource.Decode` | `Resource.decode` |
| GAP-06 | Wave from an arbitrary URL | A wave can start at a bookmarked path or URL. | `Wave.From`, `Surfer.Open` | `Wave.from_url`, `Surfer.open` |
| GAP-07 | Raw body on a page | The whole response body is available for decoding outside the neutral model. | `Page.RawBody` | `Page.raw_body` |
| GAP-08 | Wave with an expected status | A wave step can expect a refusal such as 422 and carry on. Go already supported this through `surf.Expect`. | already supported | `then(..., expect=)`, `then_submit(..., expect=)` |
| GAP-09 | JSON Schema `format` | A value that does not match `format: email` is rejected before sending. Other formats are not yet checked. | `checkFormat` in `surf/values.go` | `_check_format` in `surf/values.py` |
| GAP-10 | Duplicate `rel` | A resource can report which rels appear on more than one control. This is reporting only; it does not yet change which control wins. | `Resource.DuplicateRels` | `Resource.duplicate_rels` |

## Behaviour to know about

- Preflight runs first, before the input values are validated and before any request is built, so a
  caller without a session is told about the session rather than about a missing field. `Requires`
  is satisfied only by the surfer holding a bearer token; it does not yet check what the requirement
  is called.
- Any control with a price is refused. There is no payment flow yet, so a paid control can never be
  submitted until an x402 dialect and wallet step exist.
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

## Still open (not covered by the tests above)

- Python is synchronous only. An async client (`httpx.AsyncClient`) is a separate piece of work and needs its own retry and sleep seam.
- The credential hook for anything beyond a bearer token (signing, refresh on 401) is not designed. Today it belongs in the HTTP client's pipeline.
- C# has no per-step retry or timeout. Both belong in the `HttpClient` pipeline, so one wave cannot opt out of the surfer's policy.
- C# does not run the shared `/fixtures` conformance cases.
- Cookie sessions: only `Authorization: Bearer` is modelled.
- Redirect policy is unspecified, with no strict mode.
- Nothing sets the bearer token automatically after a sign-in response.
- The x402 payment flow itself.
- Python still lacks Explore, a command-line tool, `surftest`-style helpers and live AgentTrader
  journeys.
- Go and Python preflight do not yet apply to Explore's own skip rules, which are a separate copy of
  the same checks in `surf/explore.go`.
