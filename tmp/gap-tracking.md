# Surfdude behaviour gaps — tracking

Red repro tests live behind **`go test -tags=gap ./surf/...`** (from `go/`) and **`uv run pytest -m gap`** (from `python/`). They should **fail** until the ideal behaviour is implemented. Do not enable in CI until green.

## Run

```sh
cd go && go test -tags=gap ./surf/...
cd python && uv run pytest -m gap
```

Normal CI-style runs exclude gap tests: `go test ./surf/...` and `uv run pytest` (Python uses `addopts = "-m 'not gap'"`).

**Expected today (2026-11-02):** Go gap — **9 fail**, **1 pass** (`TestGAP08_WaveExpect422`). Python gap — **10 fail**.

| ID | Gap | Ideal (short) | Go test | Python test |
|----|-----|---------------|---------|-------------|
| GAP-01 | `Requires` on journeys | Refuse before HTTP with needs agent-session | `TestGAP01_RequiresPreflightBeforeHTTP` | `test_gap01_requires_preflight_before_http` |
| GAP-02 | `Price` on controls | Refuse before POST with payment required | `TestGAP02_PricePreflightBeforePOST` | `test_gap02_price_preflight_before_post` |
| GAP-03 | `Control.Type` non-hypermedia | Refuse Follow on non-JSON/hypermedia type | `TestGAP03_NonHypermediaTypePreflight` | `test_gap03_non_hypermedia_type_preflight` |
| GAP-04 | `Page.Read` with many items | Error unless explicitly reading first item | `TestGAP04_ReadOnListPageMustSelectItem` | `test_gap04_read_on_list_page_must_select_item` |
| GAP-05 | Read vs problems on 422 | Do not treat problem payloads as business DTO | `TestGAP05_ReadAfter422ShouldNotDecodeAsBusinessDTO` | `test_gap05_read_after_422_should_not_decode_as_business_dto` |
| GAP-06 | Wave `From(url)` | Start wave at bookmark URL, not only root | `TestGAP06_WaveFromArbitraryURL` | `test_gap06_wave_from_arbitrary_url` |
| GAP-07 | Raw body on page | Expose envelope/body for formatter-style decode | `TestGAP07_PageExposesRawResponseBody` | `test_gap07_page_exposes_raw_response_body` |
| GAP-08 | Wave + `Expect(422)` | Chain step expects refusal; page has problems | `TestGAP08_WaveExpect422` **passes today** | `test_gap08_wave_expect_422` **fails** (no `expect` on wave) |
| GAP-09 | JSON Schema `format` | Reject invalid format before send | `TestGAP09_SchemaFormatValidated` | `test_gap09_schema_format_validated` |
| GAP-10 | Duplicate `rel` on one page | Warn or error when dialect reads duplicates | `TestGAP10_DuplicateRelReported` | `test_gap10_duplicate_rel_reported` |

## Not covered as automated reds (manual / C# / infra)

- C# shared `/fixtures` conformance
- Cookie session (only Bearer today)
- Redirect strict mode
- Auto `SetBearer` after sign-in response
- x402 payment flow (covered in spirit by GAP-02)

## Scenario notes

See chat 2026-11-02 — each GAP maps to scenario / might happen / ideally sections there.

Last updated: 2026-11-02
