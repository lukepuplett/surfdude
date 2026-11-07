# Surfdude for Go

```
hypertext/   the neutral model: Resource, Control, Input, Schema, and helpful errors
dialect/     the Dialect interface and the registry that picks one per response
dialects/    simple and agenttrader media-type readers (see ../docs/dialects.md)
surf/        Surfer, Page, Follow/Submit/Item/Read, the fluent Wave, and Explore
surftest/    helpers that fail a Go test with the helpful error
cli/         the surf command line as a function a test can call (see ../docs/cli.md)
cmd/surf/    the command line: a few lines that call cli.Run
examples/    journeys against a running AgentTrader
internal/conformance/   checks a dialect against /fixtures
```

Module path `github.com/lukepuplett/surfdude/go`. It matches the repository, which was renamed from `surfdude-csharp` to `surfdude` on 2026-11-02, so `go get github.com/lukepuplett/surfdude/go@v0.4.0` needs no `go.work` or `replace`. The public proxy may serve a stale failure for a short while after the rename: `GOPROXY=direct GONOSUMDB=github.com/lukepuplett/surfdude` works around it. [Form values and prefills](../docs/model.md#form-values-for-api-writers-and-for-journeys) · [C# client](../csharp/README.md)

## Installation

### Command-line tool (`surf`)

To install the `surf` executable to your `$GOPATH/bin`:

```sh
go install github.com/lukepuplett/surfdude/go/cmd/surf@v0.4.0
```

You can also download standalone binaries for Linux, macOS, and Windows from [GitHub Releases](https://github.com/lukepuplett/surfdude/releases).

### Go client library

To use Surfdude in your Go module:

```sh
go get github.com/lukepuplett/surfdude/go/surf@v0.4.0
```

## The `surf` command line

`surf` follows a server's own links, like a journey does, and can send things as well as read them. You name what to do (a rel), never a URL or a method:

```sh
go run ./cmd/surf http://localhost:8080                                   # what does the front door offer?
go run ./cmd/surf describe http://localhost:8080 register                 # what does this form take?
go run ./cmd/surf http://localhost:8080 register email=me@example.test    # take one step
go run ./cmd/surf --dry-run http://localhost:8080 register email=me@example.test   # show the request, send nothing
go run ./cmd/surf -q data.0.token --sign-in 'credential=env:CRED' http://localhost:8080 protected
go run ./cmd/surf http://localhost:8080 register email=$A then verify-email email=$A code=123456
go run ./cmd/surf run journey.surf                                        # a journey written in a file
go run ./cmd/surf -known-rels path/to/rels.md explore http://localhost:8080
```

Values are `name=text`, `name:=JSON`, `name=@file`, `name=@-` (stdin), `name=env:VAR`, or `--secret name` (asked for with the typing hidden). Exit codes: 0 worked, 1 the server refused it, 2 the command was wrong (nothing sent), 3 could not reach the server, 4 the server failed. Secrets are never printed. See [../docs/cli.md](../docs/cli.md) for the chain syntax (`then`, `--keep`, `{name}`), the full list of rules, and what is not built yet.

`-known-rels` takes a markdown table of backticked rels (or one rel per line). Any rel the server sends that is not on the list is reported by `explore`.

## Journeys in code

For a fluent chain like the C# client, use `surf.NewWave(s).FromRoot().Then….Go(ctx)` in [surf/wave.go](surf/wave.go). In tests, `surftest` wraps each step so failures point at the line that broke:

```go
s := surftest.New(t, url, surf.WithDialects(agenttrader.New()))
home := surftest.Root(t, s)
results := surftest.Follow(t, home, "search-listings", surf.With(surf.Values{"make": "MINI"}))
problems := surftest.Refused(t, home, "create-listing", surf.Values{"reg": ""}, 422)
```

### When the server refuses a step

A step that gets a status it did not expect returns the page **and** an `*hypertext.UnexpectedStatusError`. The page always carries the status, headers and raw body, even when no dialect can read the response (a default mux 404, a proxy's error page), so it is safe to ask for rels or notes. The error carries the same `Header` and `Body`, so `errors.As` alone is enough to read `Retry-After`:

```go
_, err := page.Follow(ctx, "make-offer")
var refused *hypertext.UnexpectedStatusError
if errors.As(err, &refused) && refused.Status == 429 {
	wait := refused.Header.Get("Retry-After")
	_ = wait
}
```

### Signing in again

Give the surfer a way to get a session and a journey never has to mention tokens:

```go
signIn := func(ctx context.Context, from *surf.Page) (string, error) {
	// from is the page the server sent with the 401. If the API offers a sign-in control it is
	// right there.
	next, err := from.Submit(ctx, "sign-in", surf.Values{"credential": credential})
	if err != nil {
		return "", err
	}
	var got struct{ Token string `json:"token"` }
	return got.Token, next.Read(&got)
}
s := surftest.New(t, url, surf.WithDialects(agenttrader.New()), surf.WithSignIn(signIn))
```

The hook runs when a step is refused with a 401: it signs in and sends the same request again, once. A surfer that holds no token yet is no different, so the first 401 is what signs it in. A control that says it `requires` a session is sent like any other, because nothing in the surfer reads `requires`. Any number of steps that fail together share one sign-in. A step that says `surf.Expect(401)` is testing the refusal, so it is left alone, and the sign-in step itself can never call the hook. Without `WithSignIn` nothing changes: the server's 401 comes back as the usual `UnexpectedStatusError`.

### Recording samples from the real response

A documented sample that is typed by hand drifts. `surftest.Golden` records it from the response instead, and fails when the response changes:

```go
home := surftest.Root(t, s)
surftest.Golden(t, home, "../../doc/samples/get-home.json", surftest.Mask("data[0].expiresAt"))
```

It makes the body repeatable before comparing: the test server's address becomes `{base}` (its port changes every run), masked fields become `<masked>`, and the JSON is indented by two spaces with the server's own key order kept, so the file reads like the response and a diff shows only what changed. Run the test with `SURFDUDE_UPDATE_GOLDEN=1` to write the file, then review the diff before committing it. Without that variable it never writes, and a mismatch lists the JSON paths that differ.

See [../docs/model.md](../docs/model.md) and [../docs/dialects.md](../docs/dialects.md).

## Publishing

Module tags **`go/vX.Y.Z`** with release tag **`vX.Y.Z`**; CLI **`surf`** on [GitHub Releases](https://github.com/lukepuplett/surfdude/releases). [../docs/releasing.md](../docs/releasing.md).
