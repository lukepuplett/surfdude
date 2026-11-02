# The model

Every language implementation reads a response into the same small model. A journey sees only this, never raw JSON, so a journey written against one server's dialect runs against another's.

## Resource

What one response holds.

| Part | Meaning |
|------|---------|
| Status, Message | The HTTP status, and the server's one-line message |
| Notes | Advice the server put on the response (optional) |
| Paging | Skip, take, count and an optional total, on a list (optional) |
| Controls | Links and forms, found by **rel** |
| Items | The members of a list. Each has controls and notes of its own |
| Problems | The server's reasons for a refusal: field, code, message |
| Body | The whole response, for decoding the data |

## Control

Something to follow or submit, found by its **rel** (its relation name). A rel is the contract. A journey never names a path or an HTTP method.

| Part | Meaning |
|------|---------|
| Rel, Title | What it is for |
| Href | Where it goes. Relative addresses resolve against the page they came from |
| Method | The HTTP method. Empty means GET |
| Enctype | The media type of the body to send. Empty means the dialect's default |
| Form | True for a form that carries inputs, false for a plain link |
| Inputs | The form's fields |
| Requires | What the caller needs first, for example `agent-session` |
| IfMatch | The version it expects, sent as the `If-Match` header |
| Price | A payment the control needs (optional) |
| Type | The media type it serves, when it is not hypermedia. A crawler skips these |

## Input

One field of a form: a **name**, whether it is **required**, an optional **value**, and a **schema** (a JSON Schema fragment). The client checks `type`, `enum`, `minimum`, `maximum`, `minLength`, `maxLength` and `pattern` before it sends anything, and says what is wrong.

A dotted name such as `location.lat` becomes a nested object in a JSON body.

## Form values: for API writers and for journeys

Surfdude never scrapes a URL to discover hidden form defaults. Anything the server wants sent without the client guessing must appear on the **control** in hypermedia. The client merges that with whatever the journey supplies, then builds one HTTP request.

### If you design the API

Think in three places, not one:

| You put it on… | Use it for… | Example |
|----------------|-------------|---------|
| **`href` / `action` (fixed part)** | Where the request goes. Resource identity the server already chose. | `/shortlist`, `/registrations/7/contact` |
| **`href` / `action` (URI template)** | Named slots that become path or query when the form is submitted. | `/registrations/{status}` with `{?skip,take}` |
| **Each input's `value` on the control** | Field defaults the server already knows: hidden ids, current step, item context. Like HTML `value` or a pre-checked box. | `"listingId": "lst_seed01"` on the shortlist action for that listing |
| **Each input's `schema` (e.g. `enum`)** | What values are allowed, not which one is selected. | `"options": ["active", "inactive"]` on the simple dialect |

Rules that keep journeys small and safe:

- Put **context that changes per page or per list item** on that item's controls (each search hit carries its own `listingId` in `fields[].value`, not a single global form).
- Mark a field **required** only when the client must supply it and the server did not set `value`. If you set `value`, the client can submit with an empty map for the other fields.
- Use **URI templates** when a value belongs in the path or query by REST habit; use **JSON body fields** (AgentTrader `_actions` / POST) for the rest. The client fills templates first, then sends leftover names in the body or query according to the method.
- Do not rely on the client parsing **query strings on the link** as form state. If it is not in `inputs` / `fields` or in a `{template}` name, Surfdude will not treat it as a field.

How each shipped dialect spells a prefilled field:

| Dialect | Where the API puts a default field value | Notes |
|---------|------------------------------------------|--------|
| **agenttrader** | `"value"` on each object in `_actions[].fields` | Go and Python read this into the model. See [fixtures/agenttrader/listings-search.json](../fixtures/agenttrader/listings-search.json). |
| **simple** | `"value"` on each object in `links[].form[]` | Go, Python, and C# read it into the model and merge it on submit. Conformance: [fixtures/simple/prefilled-form.json](../fixtures/simple/prefilled-form.json). |
| **C# simple client** | Same JSON shape | `IHypertextInputControl.Value` and merge in `BuildRequest` (prefill first, then `ThenSubmit` values). |

### If you write the journey (client code)

When you `Submit(rel, values)` or `ThenSubmit(rel, values)`:

1. The client loads the control for `rel` on the current page or selected item.
2. It **merges**: start from each input's **value** on the control (when the dialect set one), then apply your map. **Your keys win** on the same name.
3. It rejects unknown names, missing required fields, and schema violations **before** HTTP.
4. It builds the URL from **href** plus template expansion, then puts remaining merged keys in the query (GET, HEAD, DELETE) or body (POST, PUT, …).

So you usually pass **only what the user or test changed**—for example `note` on shortlist—while `listingId` rides along from the hypermedia on that item's `shortlist` action.

```go
// Search: you supply search criteria. Shortlist: server prefilled listingId on the item's form.
surf.NewWave(s).FromRoot().
    Then("search-listings", surf.With(surf.Values{"make": "MINI"})).
    ThenItem(1).
    ThenSubmit("shortlist", surf.Values{"note": "nice"}).
    Go(ctx)
```

```python
s.wave().from_root().then(
    "search-listings", values={"make": "MINI"}
).then_item(1).then_submit("shortlist", {"note": "nice"}).go()
```

If you pass nothing (`Submit(rel, nil)` / empty map), the request still sends every **value** the server attached to inputs, as long as required fields are satisfied.

**Not the same as prefill:** `schema.enum` / simple `options` only constrain what you may send; they do not pick a default. **Selected** means an input **value** on the control, or an explicit entry in your submit map.

## Journeys

| Verb | What it does |
|------|--------------|
| Root | Open the front door |
| Follow(rel) | Take a link, or a form that needs nothing |
| Submit(rel, values) | Send a form |
| Item(i) | Select one member of a list, so the next step uses its controls |
| Read(into) | Decode the data into a value |
| Expect(status) | Say that a refusal such as 422 is the point of the step |

A journey that cannot take a step fails with what the page did offer: `Unable to follow "new-listing". This page has no control with that rel. It offers: self, health.` Write the journey first, then fix the API until it runs.

## Retries, timeouts and credentials

A surfer is built for two jobs. As a testing tool it should show every failure, so it retries nothing by default. As the client of a real server it may need to ride out a busy moment. The same client serves both, and the difference is a policy the caller opts into.

- Retries are off unless a policy is given, either on the surfer or on one step. A step's own policy replaces the surfer's, and a policy that never retries turns it off for that step.
- A retry is step-aware. A status the step expects is never retried, so a journey that tests a refusal such as 503 sees it exactly once. Only a failure the step did not expect, or a request that got no answer, is offered to the policy.
- A policy sees the attempt that failed: its number, the method, the status and headers or the transport error, and whether repeating the request is safe. It answers with how long to wait, or that it gives up.
- Repeating is safe for GET, HEAD, OPTIONS, PUT and DELETE, and for any request that carries an `Idempotency-Key`. A POST is not repeated unless the step asks for a key, and then only if the server honours that header. The key is generated once per step and is the same on every attempt.
- The standard preset allows three attempts in all, backs off exponentially from 200 ms to 5 s with jitter, and retries 429, 502, 503, 504 and connection errors. A `Retry-After` is obeyed in place of the backoff, but one longer than the 5 s cap counts as a refusal instead of a wait.
- The number of attempts is reported on the page and on each line of a wave's report, including the line for the step that failed.
- In Go, a cancelled context ends a backoff wait at once, and a cancelled request is never retried. Python's client is synchronous, so a wait ends only when the sleep does.
- The timeout belongs to the HTTP client. Go and Python accept one when they build their own client and otherwise leave the caller's client alone. Go changes the bearer token under a lock, so signing in on one goroutine is safe while others ride.
- Credentials sit on the surfer, not on each journey. Anything fancier than a bearer token, such as signing or token refresh, belongs in the HTTP client's pipeline.

In C#, the surfer does no retrying of its own, because `HttpClient` already has a place for it. A caller who wants retries gives the surfer a client whose handler pipeline retries, for example with Polly or `Microsoft.Extensions.Http.Resilience`. That also keeps one policy for the whole application.

## Explorer

Crawls everything reachable from the front door by following links, and forms that can be sent as a GET with nothing required. It proves **reachability**, so a route nothing links to is never visited. It reports dead links, responses no dialect can read, rels a registry does not know, and malformed controls. It never submits a form that changes state, and it skips controls that need a token it does not hold, saying so.
