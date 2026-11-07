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

### Chain

```sh
surf agenttrader.net \
  register email=$A \
  then outbox to=$A --keep 'code=data.0.text~[0-9]{6}' \
  then verify-email email=$A code='{code}' --keep cred=data.0.credential \
  then sign-in credential='{cred}'
```

Steps are joined by the word `then`. A step may end with:

- `--keep NAME=PATH` captures a value from that step's response for later steps. `PATH` is dotted, such as `data.0.token`; a number indexes a list.
- `--keep NAME=PATH~PATTERN` pulls part of the text out: the first group of the pattern if it has one, otherwise the whole match. This is how a six-digit code is taken out of an email.
- A later step uses `{NAME}` inside a `name=text` value. Only `name=text` values are templates; JSON, file, stdin and environment values are sent as they are.

Every `{NAME}` must be kept by an **earlier** step. That is checked before the first request. The chain stops at the first step that fails and exits with that step's code. Only the **last** step's response is printed.

A step looks for its rel on the response before it, and then at the front door. So `verify-email`, which the front door offers, can follow a step whose response offers nothing.

### Script file

```
# journeys/register.surf
root agenttrader.net
register email=me@example.test
outbox to=me@example.test --keep 'code=data.0.text~[0-9]{6}'
verify-email email=me@example.test code={code} --keep cred=data.0.credential
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

`--sign-in 'credential=env:CRED'` is `WithSignIn` on the command line: when a step is refused with 401, `surf` follows the `sign-in` rel (from the 401 page if it offers one, otherwise from the front door) with those values, takes the token from the response, and sends the step again **once**. `--token-path` says where the token is in the sign-in response (default `data.0.token`). The sign-in values use the same syntax as a step, but cannot be `{templates}`.

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
| 2 | the command was wrong: nothing was sent (or, for a bad `--keep` or `-q` path, the step already had been) |
| 3 | the server could not be reached, or its answer could not be read |
| 4 | the server failed (5xx) |

### Flags

`-dialects`, `-bearer`, `-bearer-env`, `-known-rels` and `-max` are as before for `show` and `explore`. New: `-json`, `-q PATH`, `-dry-run`, `-sign-in 'values'`, `-token-path PATH`. One dash or two both work.

## Secrets

Nothing a person passes as a secret is ever printed by `--dry-run`, an announcement, or an error. A value is a secret when:

- it comes from standard input (`@-`), the environment (`env:`) or `--secret`; or
- its field name contains `credential`, `token`, `password` or `secret`, in any case (so `userPassword` and `auth.token` count, and `tokenizer` is hidden too: it errs towards hiding. `agentKey` is not hidden); or
- it was built from a `{template}` filled with a value kept from such a field.

A secret shows as `<secret>` in a dry run's body and address, and the `Authorization` header shows `Bearer <secret>`. A value read from a file shows `<@path>`, so you see where it came from without the contents. Errors name a field, never its value.

The data summary of a response hides any field named like a secret too, because a terminal is read over shoulders and kept in scrollback. That means a field such as `passwordWasRight` is hidden as well. `--json` and `-q` print real values: that is how a script, or a person who means to, reads a credential.

`--secret NAME` keeps a value off the command line, out of shell history and out of the process list. At a terminal it prompts on `/dev/tty` with typing hidden (it turns echo off with `stty`, and restores it, also on Ctrl-C). If typing cannot be hidden it refuses rather than show it, and says to pipe the secret in on standard input. Not at a terminal, it reads one line of standard input, so `--secret a --secret b` read two lines in that order.

## Decisions

Made in building this, where the design did not say. Veto any of them.

1. **A step falls back to the front door.** Its rel is looked for on the last response and then on the front door. Without this a chain could not go from `outbox` to `verify-email`.
2. **`--keep NAME=PATH~PATTERN`** is an addition to the design. Without it a chain cannot take a code out of an email's text.
3. **Only the last step is printed**, and earlier steps print nothing but the `POST rel -> url` announcement of a step that changes something.
4. **Refusals go to stderr**, and a refused or failed step leaves stdout empty (except with `--json`).
5. **Exit 2 also covers a rel the server does not offer**, not only mistakes in how the command was typed, because it is something the caller asked for that is not there.
6. **Exit 3 is anything else**: a network failure, or a response no dialect can read. There is no separate code for the second.
7. **`show` and `explore` keep exit 1 for every failure,** so existing scripts are unaffected. For the new commands an unknown `-dialects` name is a usage error (2).
8. **A dry run reads the front door.** That is a GET, and it is the only request.
9. **`name=42` is the string `"42"`.** If the form says integer, the local check refuses it and points at `:=`. The command never guesses a type.
10. **Secret hiding is by name as well as by source**, and hides in the response summary too (see Secrets).
11. **The terminal prompt uses `stty`** so there is no new dependency. It works where `/dev/tty` and `stty` exist (macOS, Linux). Windows pipes the secret in.
12. **The sign-in values are read before anything is sent**, so a missing environment variable is found before the network. Standard input can feed the step or the sign-in, not both.
13. **A library addition:** `surf.Page.Prepare` builds the request a step would send without sending it. `--dry-run` needs it, and a test can use it too. `Follow` and `Submit` now share its code.
14. **No retry.** One attempt, as the library does by default.

## Not built yet

- **`surf shell ROOT`**, an interactive menu: numbered links and forms, type a number and values. It needs the step loop in `cli/run.go` split so one step can run at a time. It is the best way to learn an API by hand, and is next.
- **`--profile`**, a private file under `~/.config/surf` that keeps a base URL and a short-lived token between runs.
- Controls **on list items** (`ThenItem` in the library) cannot be named from the command line yet; a step sees only the page's own controls.
- **`--expect STATUS`**, for a step that is meant to be refused (the library has `surf.Expect`).
- **`describe --json`**, the same facts for a program.
- Windows secret prompts.
- Python and C# ports (see `tmp/gap-tracking.md`).
