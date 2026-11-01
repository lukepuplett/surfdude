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

One field of a form: a **name**, whether it is **required**, a suggested **value** (sent unless overridden, like an HTML value attribute), and a **schema**, a JSON Schema fragment. The client checks `type`, `enum`, `minimum`, `maximum`, `minLength`, `maxLength` and `pattern` before it sends anything, and says what is wrong.

A dotted name such as `location.lat` becomes a nested object in a JSON body.

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

## Explorer

Crawls everything reachable from the front door by following links, and forms that can be sent as a GET with nothing required. It proves **reachability**, so a route nothing links to is never visited. It reports dead links, responses no dialect can read, rels a registry does not know, and malformed controls. It never submits a form that changes state, and it skips controls that need a token it does not hold, saying so.
