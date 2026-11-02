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

Module path `github.com/lukepuplett/surfdude/go`. It matches the repository, which was renamed from `surfdude-csharp` to `surfdude` on 2026-11-02, so `go get github.com/lukepuplett/surfdude/go@v0.1.0` needs no `go.work` or `replace`. The public proxy may serve a stale failure for a short while after the rename: `GOPROXY=direct GONOSUMDB=github.com/lukepuplett/surfdude` works around it. [Form values and prefills](../docs/model.md#form-values-for-api-writers-and-for-journeys) · [C# client](../csharp/README.md)

## Installation

### Command-line tool (`surf`)

To install the `surf` executable to your `$GOPATH/bin`:

```sh
go install github.com/lukepuplett/surfdude/go/cmd/surf@v0.1.0
```

You can also download standalone binaries for Linux, macOS, and Windows from [GitHub Releases](https://github.com/lukepuplett/surfdude/releases).

### Go client library

To use Surfdude in your Go module:

```sh
go get github.com/lukepuplett/surfdude/go/surf@v0.1.0
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

See [../docs/model.md](../docs/model.md) and [../docs/dialects.md](../docs/dialects.md).

## Publishing

Module tags **`go/vX.Y.Z`** with release tag **`vX.Y.Z`**; CLI **`surf`** on [GitHub Releases](https://github.com/lukepuplett/surfdude/releases). [../docs/releasing.md](../docs/releasing.md).
