# Surfdude

A smart hypermedia client for RESTful APIs in Go, Python, and C#.

Surfdude drives an API the way a person drives a website: start at the front door, follow links, and submit forms. Nothing in a journey hardcodes a URL path or an HTTP method, only relation names (`rel`s). If a journey runs to completion, it proves the API is genuinely navigable by its own hypertext. If it cannot, Surfdude tells you exactly what the response offered and what control was missing.

## What is hypermedia?

`<a>` and `<form>`.

Hypermedia is the addition of self-documenting controls inside the HTTP response itself, as opposed to relying on out-of-band documentation or external metadata. In other words, it is the web as designed by Tim Berners-Lee in 1989: every response tells the client what it can do next and how to do it.

## Why build hypermedia APIs now?

Autonomous AI agents are well capable of driving an application through its API, provided the API returns the controls to make the next move (forms and links).

When an API serves true hypermedia, an agent does not need fragile hardcoded URL templates or out-of-band OpenAPI documentation that drifts out of date. The agent discovers valid actions dynamically by reading the available forms and links on each response, exactly like an online shopper moving through a website.

## The testing and feedback loop

When designing a website in HTML, you notice immediately when something is wrong: you cannot click through to get the job done because a button is missing or a form input is broken.

With a JSON API, you cannot see or click. It is easy to assume an API is usable when it actually requires guesswork or out-of-band knowledge. To find out if an API is genuinely self-navigating, you have two choices:

1. Ask an AI agent to fumble through using raw `curl` commands and burn tokens.
2. Write automated, deterministic journey tests using a tool that drives the API using only its hypertext controls.

Surfdude is that tool. If your journey test passes, your API can be driven without prior out-of-band knowledge.

## Dialects: bring your own media type

HTML has standard `<a>` and `<form>` tags. JSON does not. When designing a hypermedia API in JSON, you have to choose your own JSON representation for links, forms, and item collections.

In HTTP and REST architecture, that representation is a media type. In Surfdude, a pluggable media-type reader is called a dialect. Roy Fielding’s maxim to "design the media type" translates to choosing or implementing a dialect; the journey code itself stays the same.

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

## Getting started

You can use Surfdude in two ways. You can use the standalone command-line tool named `surf` to inspect and crawl an API from your terminal, or you can use a client library in Python, Go, or C# to write automated journeys in code.

### 1. Command-line tool (`surf`)

Installing the Python package or .NET package provides a programming library, not a terminal command. If you want a command-line tool to inspect or crawl an API immediately, use the Go executable named `surf`.

#### Installation

You can download a pre-built binary for Linux, macOS, or Windows from the [GitHub Releases](https://github.com/lukepuplett/surfdude/releases) page.

Alternatively, if you have Go installed on your system, you can install the executable directly into your Go bin directory:

```sh
go install github.com/lukepuplett/surfdude/go/cmd/surf@v0.4.0
```

#### Basic usage

The `surf` command-line tool starts at the front door of your API and inspects what hypertext controls the server offers:

```sh
# Print the links, forms, and matching dialect at the front door
surf show http://localhost:8080

# Crawl all reachable links and report any broken controls
surf explore http://localhost:8080

# Take a step: name what to do, never a URL or a method (unreleased: see docs/cli.md)
surf http://localhost:8080 register email=me@example.test
```

Besides reading, `surf` can send: one step, a chain of steps with `then`, or a journey written in a file, with secrets kept out of the output. See [docs/cli.md](docs/cli.md).

---

### 2. Python client library

The Python library lets you write automated journeys using a fluent wave builder.

#### Installation

Install the package from PyPI into your environment:

```sh
pip install surfdude==0.4.0
```

If you use `uv`, add the package to your project:

```sh
uv add surfdude==0.4.0
```

#### First journey

To drive an API, create an instance of `surfdude.Surfer` with your root URL and the dialect that matches your API media type. Then use `surfdude.Surfer.wave` to chain your steps:

```python
from surfdude import AgentTraderDialect, Surfer

# Connect to the API front door with the AgentTrader dialect
with Surfer.new("http://localhost:8080", AgentTraderDialect()) as surfer:
    # Build and execute the journey
    report = (
        surfer.wave()
        .from_root()
        .then_submit("search-listings", {"make": "MINI"})
        .then_item(0)
        .then_submit("shortlist", {"note": "nice colour"})
        .go()
    )
```

In this journey, the client sends a `GET` request to the root URL, finds the form control named `search-listings`, submits the form with the requested query parameters, selects the first search result, and submits the `shortlist` action. Nothing in the journey hardcodes a URL path because the client discovers each address from the server response.

For complete toolchain instructions, see [python/README.md](python/README.md).

---

### 3. Go client library

The Go module provides the core hypermedia engine and pluggable dialect readers.

#### Installation

Add the Go module to your project:

```sh
go get github.com/lukepuplett/surfdude/go/surf@v0.4.0
```

#### First journey

Create a client using `surf.New`, specify the dialects your API supports, and execute a wave using `surf.NewWave`:

```go
package main

import (
	"context"

	"github.com/lukepuplett/surfdude/go/dialects/agenttrader"
	"github.com/lukepuplett/surfdude/go/surf"
)

func main() {
	ctx := context.Background()

	// Initialize the client with the dialect that matches your API media type
	surfer, err := surf.New("http://localhost:8080", surf.WithDialects(agenttrader.New()))
	if err != nil {
		panic(err)
	}

	// Build and execute the wave
	report, err := surf.NewWave(surfer).
		FromRoot().
		ThenSubmit("search-listings", surf.Values{"make": "MINI"}).
		ThenItem(0).
		ThenSubmit("shortlist", surf.Values{"note": "nice colour"}).
		Go(ctx)
	if err != nil {
		panic(err)
	}

	_ = report
}
```

If any step fails, the `report` value contains details about which step broke and what controls the page actually offered. For testing helpers, see `go/surftest/` and [go/README.md](go/README.md).

---

### 4. C# (.NET) client library

The C# client targets .NET 8 and .NET 10. It drives APIs that serve the `application/simple-hypertext+json` media type through the `Evoq.Surfdude.SimpleJson` package.

#### Installation

Install the client package from your package source:

```sh
dotnet add package Surfdude.SimpleJson.Client --version 1.1.0
dotnet add package Surfdude.AgentTrader.Client --version 1.1.0
```

`Surfdude.SimpleJson.Client` reads `application/simple-hypertext+json`; `Surfdude.AgentTrader.Client` reads the AgentTrader envelope. Each brings in `Surfdude`.

For manual build and package upload instructions, see [docs/releasing.md#net--nuget-manual](docs/releasing.md#net--nuget-manual).

#### First journey

Configure an instance of `Evoq.Surfdude.Surfer` with your base address and execute a journey asynchronously:

```csharp
using System;
using System.Threading.Tasks;
using Evoq.Surfdude;
using Evoq.Surfdude.SimpleJson;

class Program
{
    static async Task Main()
    {
        // Create a reusable surfer for the API
        var surfer = Surfer.For("http://localhost:8080");

        // Execute a wave starting from the root resource
        var report = await surfer.Wave()
            .FromRoot()
            .Then("search-listings")
            .GoAsync();
    }
}
```

To configure retries, authentication headers, or logging, pass a configured `System.Net.Http.HttpClient` to `Evoq.Surfdude.Surfer.WithHttpClient`. For full details, see [csharp/README.md](csharp/README.md).

---

### 5. Choosing the right dialect for your API

A dialect is a parser that understands how a server formats links, forms, and item lists in HTTP response bodies.

Surfdude includes two primary dialects:

- `simple`: Reads the `application/simple-hypertext+json` format. It expects a JSON object containing a `links` array and optional `form` and `items` properties. Supported in Go, Python, and C#.
- `agenttrader`: Reads the AgentTrader hypermedia envelope. It expects a JSON object containing `_links`, `_actions`, and `data` arrays. Supported in Go and Python.

When you initialize a client, you pass the dialect that matches your API media type. If your API uses a different hypermedia design, you can implement the dialect interface to support your own media type. See [docs/dialects.md](docs/dialects.md) for the interface specification.

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

### Where the three clients match, and where they do not

Both dialects are available in all three languages:

| | simple | agenttrader |
|---|---|---|
| Go | in the module | in the module |
| Python | in the wheel | in the wheel |
| C# | `Surfdude.SimpleJson.Client` | `Surfdude.AgentTrader.Client` |

The features are not at parity:

| Feature | Go | Python | C# |
|---|---|---|---|
| Sign in again on a 401 | yes | yes | yes, as a `SignInHandler` in the `HttpClient` pipeline (no `Surfer.WithSignIn`) |
| Extensions bag (`Extensions`, `extensions`, `ControlData`) | yes | yes | yes |
| Retry | opt-in policy | opt-in policy | none in the library; use Polly or similar |
| Explore (crawler) and the `surf` command | yes | no | no |
| `surftest.Golden` (recorded samples) | yes | no | no |
| A response no dialect can read stays safe to inspect | yes | not yet checked | not yet checked |
| Status, notes, paging and problems on a resource | yes | yes | no: the C# model cannot hold them |
| Schema checks (`enum`, `minimum`, `pattern`, `format`) | yes | yes | only the field `type` |
| URI template expansion in AgentTrader hrefs | yes | yes | no |
| Shared conformance fixtures | all cases | all cases | AgentTrader controls only; the simple fixtures are not run |
| Live journeys against AgentTrader | yes (some skip) | no | no |
| Async client | n/a | sync only | n/a |
| 402 handling and the x402 parser | none | none | none |

Gaps are tracked in [tmp/gap-tracking.md](tmp/gap-tracking.md); release notes are in [CHANGELOG.md](CHANGELOG.md).

- **GitHub Actions CI** on **`master`** (Go, Python, C#) — see [docs/releasing.md](docs/releasing.md).
- **Tag releases** **`vX.Y.Z`**: Go module tag **`go/v…`**, PyPI via **trusted publisher**, **`surf`** on GitHub Releases; **NuGet** is manual for now.
- **Reconfigure the PyPI trusted publisher** for the renamed repository (`lukepuplett/surfdude`). The repository was renamed from `surfdude-csharp` on 2026-11-02, which is what makes the Go module path resolve. The public Go proxy may serve a stale failure for a short while; `GOPROXY=direct GONOSUMDB=github.com/lukepuplett/surfdude` works around it.
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
