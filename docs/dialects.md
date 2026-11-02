# Dialects

A server spells its links and forms however it likes. A **dialect** knows one spelling and reads it into the model in [model.md](model.md). The client never sees raw JSON, so supporting another server means writing a dialect and nothing else.

## The contract

A dialect says:

- `Name`: how it is shown in reports and errors.
- `Accept`: the media type to ask for.
- `BodyType`: the media type for request bodies when a control does not say.
- `Matches(response, body)`: can it read this? It may use headers (a profile link, a content type) and the body. It must not change either.
- `Read(response, body)`: turn the response into a Resource.

A client holds a list of dialects and tries them in order. If none matches, the error names the content type and the dialects tried.

## Shipped

| Dialect | Reads | Recognised by |
|---------|-------|---------------|
| `simple` | Surfdude's own `application/simple-hypertext+json`: `links` with an optional `form`, and `items` | The content type |
| `agenttrader` | The AgentTrader envelope: `status`, `data[]`, `paging`, `_links`, `_actions`, with controls and notes on each item | A `Link: <https://agenttrader.net/profiles/envelope/v1>; rel="profile"` header, or failing that its shape |

Go and Python ship both dialects and run `/fixtures` conformance tests. The C# client ships **simple** only today; add a formatter implementation and the same fixture files when you port **agenttrader**.

### Field defaults and URI templates (API authors)

When you add a fixture case or design a real API, spell defaults explicitly:

- **AgentTrader:** `"fields": [{"name": "listingId", "required": true, "value": "lst_01"}, …]` on `_actions`.
- **Simple:** `"form": [{"name": "skip", "isOptional": true, "value": "0"}, …]` on `links` — read by all clients; see [fixtures/simple/prefilled-form.json](../fixtures/simple/prefilled-form.json).
- **Templates:** put `{variable}` or `{?a,b,c}` in `href` / `action` for values that belong in the URL; list those names in the fixture if they are not also in `form` / `fields`.

Clients merge prefilled **values** with journey input, expand templates, then send the rest in the body or query. They do not infer hidden fields from a fully built URL string.

## Fixtures

`/fixtures/<dialect>/cases.json` lists response files, their headers, and what the model must hold after reading them. Every implementation runs the same files, so two clients are held to one definition. Add a case when you find a response a dialect gets wrong.

## Writing a dialect

1. Copy a fixture or two of the real responses into `/fixtures/<name>/` and write `cases.json` for them: the rels, which controls are forms, their methods and inputs, what is required.
2. Write the dialect so the conformance test passes.
3. Register it: `surf.WithDialects(yours.New())`.

HAL, Siren and HTML forms are natural next dialects. HTML maps well: `a` is a link and `form` is a form.
