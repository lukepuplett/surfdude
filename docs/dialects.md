# Dialects

A server spells its links and forms however it likes. In Surfdude, a **dialect** is that spelling’s **reader** on the client—the same role people describe as a **hypermedia media type** (or a **profile** on JSON): one implementation per `Content-Type` / document convention, wired through **`Accept`**, **`Matches`**, and **`Read`**.

**Dialect ≡ media type** in this documentation: the word *dialect* is what the Go and Python APIs call the plugin; *media type* is what HTTP and API design guides call the contract. The [fixtures](../fixtures/) tree is keyed by dialect name because each folder is one media-type definition.

| Term in code | HTTP / design term | C# equivalent |
|--------------|-------------------|---------------|
| `Dialect`, `WithDialects(...)` | Media type handler, `Accept` | `IHypertextResourceFormatter`, SimpleJson package |
| `Dialect.Accept()` | Request `Accept` header | Formatter default media type |
| `Dialect.BodyType()` | Default POST/PUT body type | Formatter default for bodies |
| `Dialect.Matches` | `Content-Type`, profile `Link`, shape | Chooser before read (manual today) |
| `Dialect.Read` | Parse hypermedia → model | `ReadAsResourceAsync` |

A **dialect** knows one spelling and reads it into the model in [model.md](model.md). The client never sees raw JSON for journeys; supporting another server means adding a dialect (a new media type), not changing wave code.

## The contract

A dialect says:

- `Name`: how it is shown in reports and errors.
- `Accept`: the media type to ask for.
- `BodyType`: the media type for request bodies when a control does not say.
- `Matches(response, body)`: can it read this? It may use headers (a profile link, a content type) and the body. It must not change either.
- `Read(response, body)`: turn the response into a Resource.

A client holds a list of dialects and tries them in order. If none matches, the error names the content type and the dialects tried.

## Shipped

| Dialect | Media type (primary) | Reads | Recognised by |
|---------|----------------------|-------|---------------|
| `simple` | `application/simple-hypertext+json` | `links` with optional `form`, `items` | `Content-Type` |
| `agenttrader` | `application/json` (envelope) | `status`, `data[]`, paging, `_links`, `_actions` | Profile `Link` or body shape |

Go and Python ship both dialects and run `/fixtures` conformance tests. The C# client ships **simple** only today; add a formatter implementation and the same fixture files when you port **agenttrader**.

### Field defaults and URI templates (API authors)

When you add a fixture case or design a real API, spell defaults explicitly:

- **AgentTrader:** `"fields": [{"name": "listingId", "required": true, "value": "lst_01"}, …]` on `_actions`.
- **Simple:** `"form": [{"name": "skip", "isOptional": true, "value": "0"}, …]` on `links` — read by all clients; see [fixtures/simple/prefilled-form.json](../fixtures/simple/prefilled-form.json).
- **Templates:** put `{variable}` or `{?a,b,c}` in `href` / `action` for values that belong in the URL; list those names in the fixture if they are not also in `form` / `fields`.

Clients merge prefilled **values** with journey input, expand templates, then send the rest in the body or query. They do not infer hidden fields from a fully built URL string.

Multi-step **journals** (wizards, onboarding) should put accumulated context into **`value`** and item-level controls on each response so journeys stay small—design notes in [model.md — Stateless journeys](model.md#stateless-journeys-journal-on-the-wire).

## Fixtures

`/fixtures/<dialect>/cases.json` lists response files, their headers, and what the model must hold after reading them. Every implementation runs the same files, so two clients are held to one definition. Add a case when you find a response a dialect gets wrong.

## Writing a dialect

1. Copy a fixture or two of the real responses into `/fixtures/<name>/` and write `cases.json` for them: the rels, which controls are forms, their methods and inputs, what is required.
2. Write the dialect so the conformance test passes.
3. Register it: `surf.WithDialects(yours.New())`.

HAL, Siren and HTML forms are natural next dialects. HTML maps well: `a` is a link and `form` is a form.
