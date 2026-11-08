# The surf command line

`surf` drives a hypermedia API from its front door, the way a journey does in code. You never type a URL or an HTTP method after the first argument. You name what to do (a **rel**), and `surf` finds the control with that rel on the page, fills it in and sends it. Install it with `go install github.com/lukepuplett/surfdude/go/cmd/surf@latest`, or run it from the repo with `go run ./cmd/surf` in `go/`.

This page describes the Go command. The logic is in the package `go/cli`, so it is tested without a subprocess. Python and C# do not have it yet (see `tmp/gap-tracking.md`).

## Usage

```sh
surf ROOT                                  # show what the front door offers
surf ROOT REL [name=value ...]             # take one step
surf ROOT REL ... then REL ...             # take several steps, one after another
surf describe ROOT REL                     # what does this form take?
surf run FILE                              # run a journey written in a file
surf show ROOT      surf explore ROOT      # read an API, or crawl it and report problems
```

Flags go **before** the root: `surf --json -q data.0.token ROOT ...`. `ROOT` can be written without `https://`; a host on this machine (`localhost`, `127.x.x.x`, `::1`) gets `http://`.

### One step

```sh
surf agenttrader.net register email=me@example.test
```

It opens the front door, finds the control whose rel is `register`, checks your values against what the control allows, and sends it. A step that is not a GET says what it is about to do, on stderr, before sending:

```
POST register -> https://agenttrader.net/auth/email/start
```

### Values

| You write | You send |
|---|---|
| `name=text` | the string `text` |
| `name:=JSON` | raw JSON: `count:=3`, `ok:=true`, `where:='{"lat":51.5}'`. A number keeps its exact digits |
| `name=@path` | the contents of the file (one trailing newline removed) |
| `name=@-` | all of standard input (one trailing newline removed). It can be read once |
| `name=env:VAR` | the environment variable. An unset one is an error, not an empty string |
| `--secret name` | asked for with the typing hidden (see Secrets) |
| `a.b=x` | a dot nests in a JSON body: `{"a":{"b":"x"}}` |

Only the first `=` splits, so `note=a=b` sends `a=b`. To send text that starts with `@` or `env:`, write it as JSON: `name:='"@text"'`. A mistake is refused **before anything is sent**: an unknown field, a missing required field, a value the schema does not allow, a rel the page does not offer. The message lists what is accepted. A number written as text gets a hint to use `:=`.

A rel the page does not offer is exit **5**, not 2, and the message lists what the page does offer. When exactly one offered rel is a close match (two edits or fewer away, or one contains the other, ignoring case) it adds `Did you mean "register"?`. With none close, or several, it makes no guess.

### Chain

```sh
surf agenttrader.net \
  register email=$A \
  then sandbox-outbox --keep 'code=data.0.text~[0-9]{6}' \
  then verify-email code='{code}' --keep cred=data.0.credential \
  then sign-in credential='{cred}'
```

This chain works against an API whose register response offers the mailbox, whose mailbox response offers `verify-email`, and whose verify response offers `sign-in`. If one of them does not, the chain stops there and says which control was missing (see below).

Notice what the chain does not say. The mailbox is a link with the address already in it, so `sandbox-outbox` takes no `to`: an API that wanted you to choose whose mail to read would offer a `to` field, and one that does not leaves it out. Likewise `verify-email` arrives with the email prefilled by the register response, so the step supplies only the code. A name you pass that the control does not declare is refused before anything is sent (see *Fixed and editable values* in [model.md](model.md)).

Steps are joined by the word `then`. A step may end with:

- `--keep NAME=PATH` captures a value from that step's response for later steps. `PATH` is dotted, such as `data.0.token`; a number indexes a list.
- `--keep NAME=PATH~PATTERN` pulls part of the text out: the first group of the pattern if it has one, otherwise the whole match. This is how a six-digit code is taken out of an email.
- A later step uses `{NAME}` inside a `name=text` value. Only `name=text` values are templates; JSON, file, stdin and environment values are sent as they are.

Every `{NAME}` must be kept by an **earlier** step. That is checked before the first request. The chain stops at the first step that fails and exits with that step's code. Only the **last** step's response is printed.

**A step's rel must be offered by the page it starts from**: the response before it, or for the first step the front door. A control that is not on the page you are on is the API's bug, as it would be on a website, so a chain that needs the front door's controls on a later page fails and says so:

```
Unable to follow "set-count". The front door offers it, but the last response does not (it offers: ...).
A missing control is the API's bug: each page should offer the next step.
Pass --front-door to take it from the front door anyway.
```

`--front-door` allows the fallback, and then each use prints one line to stderr (never stdout): `note: "set-count" is not on the last response, so it was taken from the front door.` A script file follows the flag as it is given.

### Script file

```
# journeys/register.surf
root agenttrader.net
register email=me@example.test
sandbox-outbox --keep 'code=data.0.text~[0-9]{6}'
verify-email code={code} --keep cred=data.0.credential
sign-in credential={cred}
```

`surf run journeys/register.surf`. The first line says where to start. Each later line is one step (a line may contain `then` and be several). Blank lines and lines starting with `#` are skipped, and quotes work as in a shell. The whole file is read before anything is sent, and an error names its line. Flags before `run` apply: `surf --json run FILE`.

### Describe and dry run

`surf describe ROOT REL` prints the fields of a control, which are required (a star), their types and limits, and a line to start from:

```
set-count  POST  https://x.test/count
fields (a star marks one that is required):
  count*  integer, min 1, max 10 - how many
  note    string, max 20 chars
try it:
  surf https://x.test set-count count:=VALUE [note=TEXT]
```

`--dry-run` shows the request the first step would send and sends nothing:

```
POST register -> https://x.test/auth/email/start
  Accept: application/json
  Content-Type: application/json
  body: {"email":"me@example.test"}
```

A dry run reads the front door (a GET) to know what the control is, and sends nothing else. For a chain it shows the first step and names the rest, because they depend on responses that do not exist yet.

### Sessions

`surf` keeps no state between runs. Send a token with `--bearer TOKEN` or `--bearer-env VAR`.

`--sign-in 'credential=env:CRED'` is `WithSignIn` on the command line: when a step is refused with 401, `surf` follows the `sign-in` rel (from the 401 page; from the front door only with `--front-door`, with the same note on stderr) with those values, takes the token from the response, and sends the step again **once**. `--token-path` says where the token is in the sign-in response (default `data.0.token`). The sign-in values use the same syntax as a step, but cannot be `{templates}`.

### Output

By default, compact and for people: the status and message, the server's notes and problems, a one-line data summary, and what can be done next (links and forms, with required fields starred).

- `--json` prints the whole response, and nothing else on stdout.
- `-q PATH` prints one value, plain (no quotes around a string), so it works in `$(...)`: `TOKEN=$(surf --sign-in ... ROOT protected -q data.0.token)`. A path that finds nothing is a usage error.
- When the server **refuses** a request, the human layout goes to **stderr** and stdout stays empty. With `--json` the response still goes to stdout, so a script can read the problems. `-q` prints nothing on a refusal.

### Exit codes

| Code | Meaning |
|---|---|
| 0 | it worked |
| 1 | the server refused the request (4xx), or `explore` found a problem. `show` and `explore` use 1 for every failure, as they always did |
| 2 | the command line was badly formed: a wrong flag, a value that cannot be read, a field the form does not have, a required field left out. Nothing was sent (or, for a bad `--keep` or `-q` path, the step already had been) |
| 3 | the server could not be reached, or its answer could not be read |
| 4 | the server failed (5xx) |
| 5 | the rel you named is not offered by the page it was looked for on (a step, `describe`, or the sign-in rel) |
| 130 | Ctrl-C while the hidden `--secret` prompt was waiting. Echo was put back first |
| 143 | a terminate signal (SIGTERM) at the same prompt |

Exit 5 is "command not found". A typo and a missing control cannot be told apart, which is why bash gives "command not found" its own code (127) instead of folding it into a usage error. So one code says the name you gave is not there, and the message lists what the page does offer, with a `Did you mean` when one is clearly close. Exit 2 is kept for a command line that is badly formed.

### Flags

`-dialects`, `-bearer`, `-bearer-env`, `-known-rels` and `-max` are as before for `show` and `explore`. New: `-json`, `-q PATH`, `-dry-run`, `-sign-in 'values'`, `-token-path PATH`, `-front-door` (let a step take its rel from the front door, with a note) `-reveal` (show values hidden by field name; see Secrets), and `-v` / `-trace` (one line per step on stderr). One dash or two both work.

Two options belong to a **step**, and go after it like `--keep`:

- `--expect 422` (or `422,429`): these statuses are the success of the step, so a step meant to be refused can be scripted and the chain carries on. Example: `surf ROOT register email=$A then register email=$A --expect 422 then ...`.
- `--unchecked`: send the values as given even where the form forbids them (a field it does not have, a missing required one, a value its schema refuses, a changed read-only one), to see whether the server refuses them too. A form's fixed values are advice, not security.

`describe` and the `next:` list mark a field the API made read-only: `describe` says `(fixed by the API: VALUE)` and the line to start from leaves it out, and `next:` writes `name[fixed]`.

## Secrets

Nothing a person passes as a secret is ever printed by `--dry-run`, an announcement, or an error. A value is hidden for one of two reasons, its **source** or its **name**.

**By source.** A value that comes from standard input (`@-`), the environment (`env:`) or `--secret` is hidden, because the person chose to hide it. A value built from a `{template}` filled with a hidden value is hidden too. `--reveal` never shows these.

**By name.** A value is hidden when its field name is on this list, which is a list of **full names**, not a guess from part of a name:

```
credential  token  accesstoken  refreshtoken  password
secret  clientsecret  apikey  privatekey  authorization
apitoken  authtoken  bearertoken  idtoken  sessiontoken  userpassword
```

(The last row was added on 2026-11-08, at Luke's word, after noticing that an API could call a token `apiToken`.) A name matches when it equals one of these after it is lower-cased and its `_` and `-` are removed, so `Credential`, `access_token`, `API-Key`, `CLIENT_SECRET` and `userPassword` match. A dotted name, such as `user.password` or a `--keep` path like `data.0.credential`, is judged by its **last part**. Names that only contain or resemble a listed one are **shown**: `passwordWasRight`, `tokenizer`, `secretary`, `keyboard`, `tokenCount` and `userPasswordHint` are not hidden, and neither is `agentKey`, which is public. If a field you care about is not on the list, give it with `--secret`, or send it from `env:` or `@-`.

**Where.** A hidden value shows as `<secret>` in a dry run's body and address, in the data summary of a response (a terminal is read over shoulders and kept in scrollback), and as a prefilled value in `describe`. The `Authorization` header always shows `Bearer <secret>`. A value read from a file shows `<@path>`, so you see where it came from without the contents. Errors name a field, never its value.

**The switch.** `--reveal` shows the values that are hidden **by name**, in the summary, in `describe`, and in a dry run. It does not show values hidden by source, and it does not show the bearer token. `--json` and `-q` always print real values: that is how a script, or a person who means to, reads a credential.

`--secret NAME` keeps a value off the command line, out of shell history and out of the process list. At a terminal it prompts on `/dev/tty` with typing hidden (it turns echo off with `stty` and restores it on every way out, including Ctrl-C and a terminate signal; see Decisions 18 and *How the Ctrl-C fix was checked*). If typing cannot be hidden it refuses rather than show it, and says to pipe the secret in on standard input. Not at a terminal, it reads one line of standard input, so `--secret a --secret b` read two lines in that order.

## Decisions

Made in building this, where the design did not say. Veto any of them.

1. **The front-door fallback is opt-in** (Luke, 2026-11-07; the first build did it silently). A step's rel must be offered by the page it starts from. Going back to the front door needs `--front-door` and prints a note on stderr. Reason: a control that is missing from the page you are on is the API's bug, as it would be on a website, and a silent fallback hides exactly the gaps this tool exists to find. The cost is that a chain must follow what each response offers, so an API has to offer the next step on each response.
2. **`--keep NAME=PATH~PATTERN`** is an addition to the design. Without it a chain cannot take a code out of an email's text.
3. **Only the last step is printed**, and earlier steps print nothing but the `POST rel -> url` announcement of a step that changes something.
4. **Refusals go to stderr**, and a refused or failed step leaves stdout empty (except with `--json`).
5. **A rel that is not offered is exit 5, apart from usage errors** (Luke, 2026-11-07; the first build used 2). A typo and a missing control cannot be told apart, like "command not found" in bash, which has its own exit code (127). So exit 5 covers both, the message lists what the page does offer, and `Did you mean` appears only when exactly one offered rel is close (two edits or fewer, or one contains the other). Exit 2 means the command line is badly formed.
6. **Exit 3 is anything else**: a network failure, or a response no dialect can read. There is no separate code for the second.
7. **`show` and `explore` keep exit 1 for every failure,** so existing scripts are unaffected. For the new commands an unknown `-dialects` name is a usage error (2).
8. **A dry run reads the front door.** That is a GET, and it is the only request.
9. **`name=42` is the string `"42"`.** If the form says integer, the local check refuses it and points at `:=`. The command never guesses a type.
10. **Secret hiding is by source and by a list of full field names, with a switch** (Luke, 2026-11-07; the first build matched part of a name, which hid `passwordWasRight` and `tokenizer`). The list is fixed (see Secrets), a dotted name is judged by its last part, and `--reveal` turns off hiding by name only. Hiding by source, and the bearer token, are never turned off. A consequence: names such as `apiToken`, `authToken` and `userPassword` are no longer hidden by name. Hide them with `--secret` or `env:`.
11. **The terminal prompt uses `stty`** so there is no new dependency. It works where `/dev/tty` and `stty` exist (macOS, Linux). Windows pipes the secret in.
12. **The sign-in values are read before anything is sent**, so a missing environment variable is found before the network. Standard input can feed the step or the sign-in, not both.
13. **A library addition:** `surf.Page.Prepare` builds the request a step would send without sending it. `--dry-run` needs it, and a test can use it too. `Follow` and `Submit` now share its code.
14. **No retry.** One attempt, as the library does by default.
15. **`--expect STATUS[,STATUS...]` is per step**, and takes codes from 100 to 599. The listed statuses are the **whole** of success, as `surf.Expect` is in the library: a status on the list is a success, the chain goes on from that page (the next rel must be offered by it, as always), and as the last step it prints normally and exits 0. A status not on the list fails as usual, a 2xx included, so `--expect 422` does not hide a 500 and does not accept a 200. A step that expects a 401 does not sign in. A bad list (no value, `abc`, `99`, `600`, `200,`, an empty one, or `--expect` given twice) is a usage error, exit 2, with nothing sent.
16. **`--unchecked` is per step** and is `surf.Unchecked()` in the library. It skips the local checks (unknown field, missing required field, schema, read-only) and sends what it is given, so the server's own validation can be probed. It prints one line to stderr before sending: `note: sending unchecked: these values are not checked against the form.` It does not unmask secrets, and `--dry-run` honours it.
17. **`-v` / `--trace` writes one line per step to stderr**, never to stdout: `step 2 sandbox-outbox GET 200 4ms` (number, rel, method, status, time). A failing step is traced with the status it got (`step 1 refuse POST 422 4ms`); a failure before any step is sent has no step line. A step that keeps values adds their **names** only (`kept=code`), never a value, and no request body, header, address query or secret is ever shown. The sign-in that `--sign-in` runs gets a `sign-in` line. The clock is `IO.Now`, so tests control it.
18. **Exit 130 and 143 only at the secret prompt.** Ctrl-C (130) or a terminate signal (143) while `--secret` waits at the terminal puts echo back, prints a newline and exits with 128 plus the signal, as a shell does. Signals at any other time keep Go's default behaviour. Echo is also restored after a line, a read error and a panic.
19. **A field that repeats a fixed `href` parameter is a problem to report, not to refuse.** `surf explore` counts it as a problem (exit 1); `surf describe` warns on stderr and still exits 0, because the mistake is the API's and the command is fine: `warning: field "to" repeats a parameter fixed in the href.` (explorer finding text: `field "to" repeats a parameter fixed in the href`).
20. **`readOnly` keeps the prefilled value.** A field the API marks `readOnly: true` is shown as fixed (`describe` says `(fixed by the API: VALUE)`; the `next:` list marks it `name[fixed]`; the line to start from leaves it out) and a step that gives a different value is refused locally with exit 2 and no value in the message. The same value, or none, is fine. `--unchecked` overrides. A read-only field with no prefilled value accepts nothing.
21. **A library addition:** `surf.Unchecked()`, `hypertext.Input.ReadOnly()`, `hypertext.Control.RepeatedFixedParams()`, `*hypertext.ReadOnlyInputError`, and `surftest.RefusedPage` (like `Refused`, and returns the refused page so a test can read its notes or follow what it offers).

### How the Ctrl-C fix was checked

The prompt is in `go/cmd/surf/prompt.go` behind a small `terminal` interface, so unit tests (`prompt_test.go`) drive it with a fake that blocks, panics and takes a signal. A fake cannot show that the **real** terminal's echo comes back, so that was checked by hand under a real pseudo-terminal: a Python `pty.fork` harness ran the built `surf` as `surf http://127.0.0.1:1 sign-in --secret credential` inside `sh` (which ignored INT and TERM, so only `surf` reacted), read the terminal's echo flag with `termios` while the prompt waited, then did one of three things and ran `stty -a` afterwards:

| Case | Echo while waiting | surf exit code | Echo afterwards |
|---|---|---|---|
| typed `abc` and Enter | off | 3 (the front door at port 1 is unreachable) | on |
| Ctrl-C (byte `\x03` on the pty) | off | 130 | on |
| `SIGTERM` to the process group | off | 143 | on |

This is a manual check on macOS, not an automated test; re-run it if `prompt.go` or `secret.go` changes.

22. **The hidden prompt waits for its signal listener to stop before it returns** (2026-11-08). A test failed 46% of the time: closing a channel told the listener to stop but did not wait, so a signal arriving just after the prompt returned could still be picked by the listener's `select`, which chooses at random when two channels are ready, and exit with 130 after the secret had been read. The prompt now waits for the listener to exit, and the test passes 1000 runs in a row and 100 under the race detector.
23. **Six more names are hidden by name** (2026-11-08, Luke): `apitoken`, `authtoken`, `bearertoken`, `idtoken`, `sessiontoken` and `userpassword`, after noticing an API could call a token `apiToken`.

## Not built yet

- **`surf shell ROOT`**, an interactive menu: numbered links and forms, type a number and values. It needs the step loop in `cli/run.go` split so one step can run at a time. It is the best way to learn an API by hand, and is next.
- **`--profile`**, a private file under `~/.config/surf` that keeps a base URL and a short-lived token between runs.
- Controls **on list items** (`ThenItem` in the library) cannot be named from the command line yet; a step sees only the page's own controls.
- **`describe --json`**, the same facts for a program.
- Windows secret prompts.
- Python and C# ports (see `tmp/gap-tracking.md`).
