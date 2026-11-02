# Surfdude

Because you can surf the best APIs.

A hypermedia client that **drives an API the way a person drives a website**: start at the front door, follow links, submit forms. Nothing in a journey names a URL or an HTTP method, only link names (`rel`s). So a journey that runs proves the API really can be driven by its own hypertext, and one that cannot says what the page did offer.

## Dialect = media type

In Surfdude, a **dialect** is the client-side name for a **hypermedia media type**: one pluggable reader for one way the server spells links and forms in HTTP responses (and usually one primary **`Content-Type`** / **`Accept`** pair). Roy Fielding’s “design the media type” is “ship or choose a dialect”; the wave/journey code stays the same.

| Dialect (name) | Typical `Accept` / recognition | JSON shape (short) | In this repo |
|----------------|----------------------------------|--------------------|--------------|
| **simple** | `application/simple-hypertext+json` | `links` + optional `form`, `items` | Go, Python, C# ([SimpleJson](csharp/)) |
| **agenttrader** | `application/json` + AgentTrader profile `Link` (or body shape) | envelope: `_links`, `_actions`, `data[]` | Go, Python |

The registry tries dialects in order until **`Matches`** succeeds, then **`Read`** fills the neutral [model](docs/model.md). Request bodies use the dialect’s **`BodyType`** when a control does not set `enctype`. In C#, the same idea is **`IHypertextResourceFormatter`** (today **`Evoq.Surfdude.SimpleJson`** = **simple** only). Full contract: [docs/dialects.md](docs/dialects.md).

## What is in here

| Folder | What |
|--------|------|
| [csharp/](csharp/) | The original client (**simple** dialect). **.NET 8** / **.NET 10** — [csharp/README.md](csharp/README.md) |
| [go/](go/) | The Go client: same journey verbs, pluggable **dialects** (the examples use **agenttrader**; C# below uses **simple**). Explorer and CLI in [go/](go/) |
| [python/](python/) | Python client: same fixtures and model; **uv** + **pytest** — see [python/README.md](python/README.md) for toolchain |
| [docs/](docs/) | [model.md](docs/model.md): neutral model, journeys, form values, stateless journals. [dialects.md](docs/dialects.md): **dialect = media type**, registry, fixtures |
| [fixtures/](fixtures/) | Responses and expectations that every implementation must agree on |
| [scripts/](scripts/) | `journey-agenttrader.sh` starts a local AgentTrader and drives it |
| [tmp/gap-tracking.md](tmp/gap-tracking.md) | Behaviour gaps found in review, what fixed each one, and what is still open |
| [docs/releasing.md](docs/releasing.md) | **Tag `vX.Y.Z`** → Go module, PyPI (OIDC), **`surf`** on GitHub Releases; NuGet manual — [full guide](docs/releasing.md) |

## Install (after a release)

See [docs/releasing.md](docs/releasing.md) for tagging, PyPI trusted publisher, and NuGet manual upload. Current line **0.1.0** (see [VERSION](VERSION)):

| | Command |
|---|---------|
| **Go library** | `go get github.com/lukepuplett/surfdude/go/surf@v0.1.0` |
| **Go CLI** | `go install github.com/lukepuplett/surfdude/go/cmd/surf@v0.1.0` or download **`surf`** from [GitHub Releases](https://github.com/lukepuplett/surfdude-csharp/releases) |
| **Python** | `uv add surfdude==0.1.0` or `pip install surfdude==0.1.0` (after Release workflow with OIDC has run for that tag) |
| **.NET** | `dotnet add package Surfdude.SimpleJson.Client --version 0.1.0` — build/upload via [docs/releasing.md](docs/releasing.md#net--nuget-manual) until you publish to nuget.org |

Automated publish today: **Go**, **PyPI**, **GitHub Release** binaries. **NuGet:** local `./scripts/tag-release/build-local.sh` then upload.

## Go sample ride

The Go examples drive a running **AgentTrader** using the **agenttrader** dialect (`agenttrader.New()` — the AgentTrader **media type**). The C# sample below uses the **simple** dialect (`application/simple-hypertext+json`). Same journey verbs; different **Accept** and JSON spelling. See [Dialect = media type](#dialect--media-type) and [docs/dialects.md](docs/dialects.md).

Express the journey as a **wave** (like the C# client). Nothing runs until `Go`; if a step fails you get a **Report** showing how far it got.

```go
ctx := context.Background()
s, _ := surf.New("http://localhost:8080", surf.WithDialects(agenttrader.New()))

_, err := surf.NewWave(s).
	FromRoot().
	ThenSubmit("search-listings", surf.Values{"make": "MINI"}).
	ThenItem(0).
	ThenSubmit("shortlist", surf.Values{"note": "nice colour"}).
	Go(ctx)
```

That matches JSON the **agenttrader** dialect reads: an envelope with `_links`, `_actions`, and typed `fields` on each action (full cases in [fixtures/agenttrader/](fixtures/agenttrader/)). The client picks the dialect from a `Link: …/profiles/envelope/v1`; rel=`profile` header, or from the body shape.

```json
{
  "_links": [
    {"rel": "self", "href": "/", "method": "GET"},
    {"rel": "new-listing", "href": "/listings/new", "method": "GET", "title": "Get the form to list a car for sale"}
  ],
  "_actions": [
    {
      "rel": "search-listings",
      "title": "Search live listings",
      "action": "/listings",
      "method": "GET",
      "fields": [
        {"name": "make", "required": false, "schema": {"type": "string"}},
        {"name": "maxPrice", "required": false, "schema": {"type": "integer"}}
      ]
    }
  ]
}
```

```sh
cd go && go run ./cmd/surf show http://localhost:8080   # prints which dialect read the page
cd go && go run ./cmd/surf explore http://localhost:8080   # crawl everything reachable
```

Design an API by writing the journey first. The errors tell you what to add to the hypertext until the journey runs. Multi-step flows work best when each response **prepopulates** the next forms (`value` on inputs, controls on list items)—see [Stateless journeys: journal on the wire](docs/model.md#stateless-journeys-journal-on-the-wire). Field mechanics: [Form values](docs/model.md#form-values-for-api-writers-and-for-journeys). In tests, step-by-step helpers live in [go/surftest/](go/surftest/); see [go/README.md](go/README.md).

## Python sample ride

Same journey and **agenttrader** dialect as Go, with a fluent **wave** (`go()` runs the chain). Install and test with [uv](https://docs.astral.sh/uv/) — see [python/README.md](python/README.md).

```python
from surfdude import AgentTraderDialect, Surfer

with Surfer.new("http://localhost:8080", AgentTraderDialect()) as s:
    s.wave().from_root().then_submit(
        "search-listings", {"make": "MINI"}
    ).then_item(0).then_submit(
        "shortlist", {"note": "nice colour"}
    ).go()
```

```sh
cd python && uv sync --all-groups && uv run pytest   # same /fixtures cases as Go
```

## Driving a real server

By default a surfer retries nothing, so a test sees every failure. A client of a real server can opt in to retries. A retry never repeats a status the step expects, and repeats a POST only when the step carries an idempotency key. The rules are in [docs/model.md](docs/model.md#retries-timeouts-and-credentials).

```go
s, _ := surf.New(root, surf.WithDialects(agenttrader.New()),
    surf.WithRetry(surf.StandardRetry()), surf.WithTimeout(10*time.Second))

surf.NewWave(s).FromRoot().
    ThenSubmit("shortlist", surf.Values{"note": "nice colour"}, surf.WithIdempotencyKey()).
    Go(ctx)
```

```python
from surfdude import AgentTraderDialect, StandardRetry, Surfer

with Surfer.new(root, AgentTraderDialect(), retry=StandardRetry(), timeout=10.0) as s:
    s.wave().from_root().then_submit(
        "shortlist", {"note": "nice colour"}, idempotency_key=True
    ).go()
```

C# keeps one long-lived `Surfer`, so the `HttpClient` is shared rather than created for every wave. Retries, timeouts and authentication go in that client's handler pipeline, which is where .NET already puts them. What belongs to one journey is set on that wave and never leaks into the next.

```csharp
// Once, for example as a singleton in dependency injection.
var surfer = Surfer.For("https://api.example.com/")
    .WithHttpClient(httpClientWithRetryHandler)
    .WithLoggerFactory(loggerFactory);

// Per journey.
await surfer.Wave(c => c.ExpectedStatusCodes = new[] { 200, 404 })
    .FromRoot().Then("find-listing").GoAsync();
```

`Surf.Wave(rootUri)` still works as before, and now shares one default `HttpClient` instead of creating one per wave.

## Status and what is next

The Go client is new (November 2026): two dialects, an explorer, a command-line tool and first journeys against AgentTrader. The C# client is the original. It now has a reusable `Surfer`, its first unit tests in [csharp/tests/Evoq.Surfdude.Tests](csharp/tests/Evoq.Surfdude.Tests), and targets **.NET 8** and **.NET 10** (apps and tests on **.NET 10** LTS).

- **GitHub Actions CI** on **`master`** (Go, Python, C#) — see [docs/releasing.md](docs/releasing.md).
- **Tag releases** **`vX.Y.Z`**: Go module tag **`go/v…`**, PyPI via **trusted publisher**, **`surf`** on GitHub Releases; **NuGet** is manual for now.
- **Rename this repository** from `surfdude-csharp` to `surfdude`. The Go module path (`github.com/lukepuplett/surfdude/go`) already assumes it. Until then, use a `go.work` or a `replace`.
- **Teach the C# client the AgentTrader envelope.** It already has the seam (`IHypertextResourceFormatter`); the shared [fixtures](fixtures/) say what a correct reading looks like.
- **A small dialect for x402 `402` bodies,** which are not the AgentTrader envelope.
- **More dialects:** HAL, Siren, and HTML (`a` is a link, `form` is a form).
- **The C# model** is thinner than the Go one: inputs have a name and an optional flag, with no schema. It can grow to match [docs/model.md](docs/model.md).

---

<a id="c"></a>
# C# (the original README)

Build, test, target frameworks, and the `Surfer` API are documented in [csharp/README.md](csharp/README.md). The rest of this section is the original product description.

A simple little hypermedia client that dynamically drives a REST API. You can use this to test that an API is genuinely RESTful, i.e. it can be driven entirely by links and hypertext transfer, or you can use this in your application to work a backend API instead of a static URL template based client like Refit.

## Surfdude helps you build radically better APIs

#### Designing APIs by writing the desired client code first tends to drive out much nicer APIs

This is because Surfdude emits helpful errors when it can't find the information that it needs in the hypertext to perform the next step. This lets you fix and tweak the API and retry the Surfdude journey until each error is gone and the entire "wave" of API interactions can be surfed successfully.

When I design an API, I tend to use a Surfdude client in a little console app and then work on an API Blueprint using Apiary and run my Surfdude test client against the Apiary mock until it looks and feels right, then I start coding the real API before pointing my Surfdude client at the real one to check it through.

Not only do I end up with a proper RESTful API that's driven dynamically, but using the API from Postman is better since the API is fully self-documenting. There's no need for Swagger or anything else, just as a good website doesn't come with instructions.

You could potentially even replace the code that executes the steps with code that generates the JSON that would make the step work. This would literally let your API client write its own API Blueprint, which can then be used to scaffold code.

Out-of-the-box, Surfdude uses its built-in library for reading and understanding hypertext controls and formulating HTTP request. This will force your APIs to adhere to a particular hypertext document or media-type that I've invented. However, you can replace these parts for some code that works with a HAL derivative or even HTML.

## API UX and Riding Interaction Waves

The premise is that, like a website, a well designed API is centred around the client's needs and problems much like how web and interaction designers build the experiences we all enjoy on the web. Mostly.

For an API this means thinking of the things that a client would like to do with the API, and their journey through the API to get those things done. At each step of the journey there is opportunity to feedback and inform the client of the impact of any changes and to present alternative options and possible next interactions.

## Sample Ride

Surfdude lets you express an interaction with an API as a journey, like riding on a wave, changing direction.

#### Start from the root / resource

Bookmarking a page on the web is risky since the website owner could move things around. So we all tend to shop by going in the front door. This is true of a REST API and it allows the author to move the URLs around and introduce new paths and trial things without breaking existing clients. This is the true power of _real_ REST.

	SurfReport report = await Surf.Wave("https://private-ac89c-surfdude.apiary-mock.com/")
		.FromRoot()
		.Then("registrations")
		.ThenItem(0)
		.ThenSubmit("update-contact-details", new { email = "mike@beans.com" })
		.ThenRead(out Func<ResourceModel> getContactDetails)
		.Then("registration")
		.ThenSubmit("add-processing-instruction", new { message = "The applicant is partially sighted." })
		.GoAsync();

	Console.WriteLine(getContactDetails().Email);

Notice how the protocol methods and resource identifiers are completely absent, reinforcing the notion that URLs cannot themselves be RESTful; a machine cares not for your wonderfully pluralised nouns and semantic structure, just as an online shopper cares not for the URL sitting beneath a button click.

## Surfdude Media-Type

The sample ride above works with JSON returned from the API that looks somewhat like this. Each link contains the information to make the next HTTP request. Optional `form` entries name inputs; you can add a **`value`** on an input when the server already knows what to send (see [docs/model.md](../docs/model.md#form-values-for-api-writers-and-for-journeys)). **`options`** (or schema `enum`) list allowed values but do not select one unless you also set **`value`** or the client passes it in `ThenSubmit`.

        {
            "links":
            [
                {
                    "rel": "registrations",
                    "description": "View the registrations progressing and complete.",
                    "href": "https://private-ac89c-surfdude.apiary-mock.com/registrations",
                    "form":
                    [
                        { "name": "skip", "label": "The number of items to skip over.", "isOptional": true },
                        { "name": "take", "label": "The number of items to return.", "isOptional": true },
                    ]
                },
                {
                    "rel": "registration-finder",
                    "description": "Lets an applicant find their registration in the system.",
                    "href": "https://private-ac89c-surfdude.apiary-mock.com/registrations/{status}",
                    "form":
                    [
                        { "name": "status", "options": [ "active", "inactive", "any" ] },
                        { "name": "email" }
                    ]
                }
            ]
        }

## Bring Your Own Media-Type

As Roy Fielding explained, most of the effort in designing a REST API should go into designing the media-type.

The JSON, HTML, XML or YAML or whatever of your API should contain hypermedia controls that contain all the information needed for the client to make use of those controls. For HTML the `<a>` and `<form>` tags come with clear processing instructions for web browser clients to make use of these controls whenever they are encountered.

The upshot of this uniformity is that a nice website needs no further documentation; if the browser cannot work with the HTML then there is a bug with the website or "web design" as we call it. If a website needs a user manual then there is a serious UX issue. The same is true of an API.

Surfdude lets you replace the classes that read and process your hypertext resources and their controls so you can design your own media-type schema.

## Evolving APIs

Websites change over time and yet customers rarely notice. Their experience remains uninterrupted, the links keep working and adding an item to a shopping cart continues to work, even though the underlying resources may be changing each day.

A great API using a dynamic client affords the same flexibility since interactions are loosely based on relation names, not tied to URLs.
