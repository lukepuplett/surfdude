# Changelog

Go and Python share one version. The .NET packages have their own, because they were already on 1.x on nuget.org (`Surfdude` and `Surfdude.SimpleJson.Client` 1.0.3) when the others were on 0.x. While a major version is 0, a minor release can break the API, and each break is listed first.

## 0.6.0

### Added

- **Go: pluggable challenge handlers** — `surf.Challenge`, `surf.WithChallenges`, `surf.ChallengeContext`. After `finish` on an unexpected status, registered matchers may run a handler; on success the surfer **rewinds and retries the request once** (same pattern as `WithSignIn`).
- **Go: x402 vs envelope 402** — `surf.MatchX402Payment`, `surf.IsX402Body`; `agenttrader.MatchCapabilityGap(status, missing)` for AgentTrader envelope `missing` (requires profile `Link`, excludes x402 bodies).
- **Go: envelope `missing`** on `hypertext.Resource` (agenttrader dialect).
- **Python: pluggable challenge handlers** — `Challenge`, `Surfer.new(..., challenges=)`, `ChallengeContext`; same rewind-and-retry-once behaviour as Go after an unexpected status.
- **Python: x402 vs envelope 402** — `surf.x402_match.match_x402_payment`, `is_x402_body`; `dialects.agenttrader_match.match_capability_gap` and envelope probe helpers.
- **Python: envelope `missing`** on `hypertext.Resource` (agenttrader dialect).
- **C#: pluggable challenge handlers** — `Challenge`, `Surfer.WithChallenges` / `AgentTraderSurfer.WithChallenges`, `ChallengeContext` on `SurfContext.Challenges`; `HttpStep` runs matchers and replays the captured request once.
- **C#: x402 vs envelope 402** — `X402Match`; `AgentTraderMatch.MatchCapabilityGap` and envelope probe helpers.
- **C#: envelope `missing`** on `AgentTraderResource` (agenttrader dialect).

## 0.5.0

The first release where the `surf` command can send, not only read (Go only; the Python package is republished at 0.5.0 with no code change, because Go and Python share one version). Also fixes the GoReleaser repository name, which still said `surfdude-csharp` after the rename.

### Added

- **Go: the `surf` command line can send, not only read.** `surf ROOT REL name=value ...` finds the control with that rel on the page it starts from (the front door for the first step), checks the values against it and sends it, so you name what to do and never a URL or a method. Steps chain with the word `then`, passing values forward with `--keep NAME=PATH[~PATTERN]` and `{name}`. `surf run FILE` runs a journey written one step a line. `surf describe ROOT REL` prints a form's fields and limits as a fill-in template, and `--dry-run` shows the request a step would send and sends nothing. `--sign-in 'name=value'` signs in on a 401 and tries the step again once.
- **Values** are `name=text`, `name:=JSON`, `name=@file`, `name=@-` (standard input), `name=env:VAR` and `--secret name` (asked for with the typing hidden). A mistake is refused before anything is sent, with what is accepted.
- **A step's rel must be offered by the page it starts from.** A control missing from the page you are on is the API's bug, so a later step that only the front door offers fails and says so. `--front-door` allows the fallback and prints a note on stderr each time it is used.
- **Output** is compact for people by default, `--json` for the whole response, `-q PATH` for one value. **Exit codes:** 0 worked, 1 the server refused, 2 the command line was badly formed, 3 the server could not be reached, 4 the server failed, 5 the rel named is not offered by the page. A typo and a missing control cannot be told apart, so exit 5 covers both, like bash's "command not found", and the message lists what the page offers and suggests a close match.
- **Secrets are never printed.** Values from standard input, the environment or `--secret` are hidden by their source. Values under a fixed list of full field names (`credential`, `token`, `accesstoken`, `refreshtoken`, `password`, `secret`, `clientsecret`, `apikey`, `privatekey`, `authorization`, `apitoken`, `authtoken`, `bearertoken`, `idtoken`, `sessiontoken`, `userpassword`; case, `_` and `-` ignored; a dotted name by its last part) are hidden by name, in a dry run, in a response summary and in `describe`. `passwordWasRight` and `tokenizer` are not hidden. `--reveal` shows what is hidden by name, never what is hidden by source or the bearer token. See [docs/cli.md](docs/cli.md).
- **Go: `surf.Page.Prepare`** builds the request a step would send without sending it. `Follow` and `Submit` share its code.
- The command's logic is the new package `go/cli`, so it is tested without a subprocess. `show` and `explore` are unchanged and keep their output and exit codes.
- **Go CLI: `--expect STATUS[,STATUS...]`** per step makes those statuses the step's success, so a step meant to be refused can be scripted and the chain carries on. **`--unchecked`** per step sends values the form forbids, to test the server's own validation, and says so on stderr. **`-v` / `--trace`** prints one line per step on stderr (`step 2 sandbox-outbox GET 200 4ms`), with kept names and never a value.
- **Go CLI: Ctrl-C at the hidden `--secret` prompt now restores terminal echo** (also on SIGTERM, a read error and a panic) and exits 130 (143 for SIGTERM). Checked under a real pseudo-terminal as well as with a fake one.
- **Go library: `surf.Unchecked()`** (skips the local checks of a step), **`readOnly` is read** into `hypertext.Input` (`Input.ReadOnly()`, both the agenttrader and simple dialects): a read-only input keeps its prefilled value and a different caller value is refused with `*hypertext.ReadOnlyInputError`. **`Control.RepeatedFixedParams()`**: `Explore` reports a field that repeats a parameter fixed in the `href` as a `bad-control` finding, and `surf describe` warns. **`surftest.RefusedPage`** returns the refused page, for a test that reads its notes or follows what it offers.
- Not built: `surf shell`, `--profile`, `describe --json`, and the Python and C# versions. See `docs/cli.md`.

## 0.4.0

0.3.0 was prepared but never tagged or published, so there is no 0.3.0 release: its changes are in this one. The last published versions were Go 0.2.0 and PyPI 0.1.0.

### Breaking

**A control keeps only what changes the request. Everything else is an extension.**

- A control now has `Rel`, `Href`, `Method`, `Enctype`, `Inputs` and `IfMatch`. Anything else a dialect finds on it (`requires`, `price`, `type`, `title`, and any other key) is kept as an extension under the dialect's own name, with the server's value as sent. The surfer never reads extensions. The rule and a per-language table are in [docs/model.md](docs/model.md#core-parts-and-extras).
- **Go:** `Control.Requires`, `Price`, `Type` and `Title` are removed, and so is the `hypertext.Price` type. Use `Control.Extensions`, `Extension(name)` and `DecodeExtension(name, &out)`.
- **Python:** `Control.requires`, `price`, `type` and `title` are removed, and so is `surfdude.hypertext.Price`. Use `Control.extensions`, a dict.
- **C#:** `IHypertextControl` gains `Method`, `Enctype` and `IfMatch`. `ControlData` now holds extras only. The new members have defaults that still read the old `method` and `if-match` keys, so existing implementations keep working.
- **No more preflight (Go and Python).** The surfer used to refuse a control before sending when it said it required a session, had a price, or served non-JSON. It now sends the request and the server's answer decides: a 401 or 402 comes back as `UnexpectedStatusError`, and an answer no dialect can read as `UnsupportedMediaTypeError`. The errors `RequiresNotMetError`, `PaymentRequiredError` and `NonHypermediaControlError` are removed.
- **Signing in:** with `WithSignIn` (Go) or `sign_in=` (Python), a surfer that holds no token no longer signs in before its first request. The first 401 signs it in, so there is one extra request.
- **Shared fixture:** `fixtures/agenttrader/cases.json` now expects `extensions.requires`, not `requires`.

### Added

- **Sign in again, in Python and C#.** Python: `Surfer.new(..., sign_in=fn)`. C#: `SignInHandler`, a handler for the `HttpClient` pipeline. On a 401 they sign in, keep the token and send the same request once more with the body intact. Concurrent requests share one sign-in, a step that expects 401 is left alone, and the sign-in request cannot call the hook again. A hook that fails, or returns no token, fails the step with `SignInFailedError` or `SignInFailedException`. Go has had this since 0.2.0.
- **`Surfdude.AgentTrader.Client`, a new NuGet package.** C# can now read the AgentTrader envelope and build the requests its controls describe: prefilled values, JSON bodies, query strings, `If-Match`, dotted names, typed numbers. Use `AgentTraderSurfer.For(url)`. The shared AgentTrader fixtures run in C# for the first time. Not yet in C#: status, notes, paging and problems, schema checks beyond `type`, and URI template expansion.
- `docs/model.md` explains core parts and extras.
- `scripts/tag-release/bump-version.sh` now works on macOS.

### Fixed

- **C#:** a control's HTTP method was lowercased (`post`, `get`). It is now uppercased.

### .NET packages: 1.1.0, not 0.4.0

The three NuGet packages ship as **1.1.0**: `Surfdude`, `Surfdude.SimpleJson.Client` (both new versions of existing packages) and `Surfdude.AgentTrader.Client` (new). 1.1.0 is a minor bump because the C# API only grows: `IHypertextControl` gains `Method`, `Enctype` and `IfMatch` with defaults that read the old keys, and `SignInHandler` and the AgentTrader package are new. One behaviour changes: on the simple dialect `ControlData` is now empty, because the method and version moved to the new members. Read `Method` and `IfMatch` instead.

### Upgrading

- Replace reads of `Control.Requires`, `Price`, `Type` and `Title` with the extension of the same name. In Go: `var requires []string; c.DecodeExtension("requires", &requires)`. In Python: `c.extensions.get("requires")`.
- Remove any handling of `RequiresNotMetError`, `PaymentRequiredError` and `NonHypermediaControlError`. Handle the 401, 402 or unsupported media type the server returns instead.
- If you pass `WithSignIn` or `sign_in=`, nothing else changes. Expect the first request to a protected control to be refused once before it signs in.

## 0.2.0 (Go only)

Go module tag `go/v0.2.0`. Added `WithSignIn`, `surftest.Golden`, and a safe, inspectable refused step (no nil `Resource`; headers and body kept). Python was not released.

## 0.1.0

First release (tag `v0.1.0`): the Go module, the `surf` command and PyPI `surfdude` 0.1.0.
