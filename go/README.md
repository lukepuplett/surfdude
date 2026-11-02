# Surfdude for Go

```
hypertext/   the neutral model: Resource, Control, Input, Schema, and helpful errors
dialect/     the Dialect interface and the registry that picks one per response
dialects/    simple and agenttrader media-type readers (see ../docs/dialects.md)
surf/        Surfer, Page, Follow/Submit/Item/Read, the fluent Wave, and Explore
surftest/    helpers that fail a Go test with the helpful error
cmd/surf/    the command line: show and explore
examples/    journeys against a running AgentTrader
internal/conformance/   checks a dialect against /fixtures
```

Module path `github.com/lukepuplett/surfdude/go`. It matches the repository, which was renamed from `surfdude-csharp` to `surfdude` on 2026-11-02, so `go get github.com/lukepuplett/surfdude/go@v0.2.0` needs no `go.work` or `replace`. The public proxy may serve a stale failure for a short while after the rename: `GOPROXY=direct GONOSUMDB=github.com/lukepuplett/surfdude` works around it. [Form values and prefills](../docs/model.md#form-values-for-api-writers-and-for-journeys) · [C# client](../csharp/README.md)

## Installation

### Command-line tool (`surf`)

To install the `surf` executable to your `$GOPATH/bin`:

```sh
go install github.com/lukepuplett/surfdude/go/cmd/surf@v0.2.0
```

You can also download standalone binaries for Linux, macOS, and Windows from [GitHub Releases](https://github.com/lukepuplett/surfdude/releases).

### Go client library

To use Surfdude in your Go module:

```sh
go get github.com/lukepuplett/surfdude/go/surf@v0.2.0
```

## Try the CLI tool on a running server

```sh
go run ./cmd/surf show http://localhost:8080
go run ./cmd/surf -known-rels path/to/rels.md explore http://localhost:8080
```

`-known-rels` takes a markdown table of backticked rels (or one rel per line). Any rel the server sends that is not on the list is reported.

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
	// from is the page that showed the need: the page whose control requires a session, or the
	// page the server sent with a 401. If the API offers a sign-in control it is right there.
	next, err := from.Submit(ctx, "sign-in", surf.Values{"credential": credential})
	if err != nil {
		return "", err
	}
	var got struct{ Token string `json:"token"` }
	return got.Token, next.Read(&got)
}
s := surftest.New(t, url, surf.WithDialects(agenttrader.New()), surf.WithSignIn(signIn))
```

The hook runs in two situations. When a control needs a session and the surfer holds no token, it signs in first instead of refusing the step. When a step is refused with a 401, it signs in and sends the same request again, once. Any number of steps that fail together share one sign-in. A step that says `surf.Expect(401)` is testing the refusal, so it is left alone, and the sign-in step itself can never call the hook. Without `WithSignIn` nothing changes: a step that needs a session with no token is refused before any request is sent.

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
