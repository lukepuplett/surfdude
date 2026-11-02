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
