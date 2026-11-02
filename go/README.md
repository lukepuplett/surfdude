# Surfdude for Go

```
hypertext/   the neutral model: Resource, Control, Input, Schema, and helpful errors
dialect/     the Dialect interface and the registry that picks one per response
dialects/    simple (Surfdude's own media type) and agenttrader (the AgentTrader envelope)
surf/        Surfer, Page, Follow/Submit/Item/Read, the fluent Wave, and Explore
surftest/    helpers that fail a Go test with the helpful error
cmd/surf/    the command line: show and explore
examples/    journeys against a running AgentTrader
internal/conformance/   checks a dialect against /fixtures
```

Module path `github.com/lukepuplett/surfdude/go`. It matches the repository once that is renamed from `surfdude-csharp` to `surfdude`. Until then use a `go.work` or a `replace`.

## Try it on a running server

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
