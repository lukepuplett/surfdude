# Surfdude for C#

The original Surfdude client. It drives APIs by **rel** using the **simple** media type (`application/simple-hypertext+json`) via **`Evoq.Surfdude.SimpleJson`** — the C# name for the **simple** dialect. Go/Python also ship **agenttrader**. See [Dialect = media type](../README.md#dialect--media-type).

## Toolchain

| What | Version |
|------|---------|
| NuGet packages (`Evoq.Surfdude`, `Evoq.Surfdude.SimpleJson`) | **net8.0** and **net10.0** |
| Sample console app and unit tests | **net10.0** (.NET 10 LTS) |

You need the [.NET SDK](https://dotnet.microsoft.com/download) **8.0 or later** (10.x recommended). The SDK builds every target framework in the solution.

Consumers must run on **.NET 8+**. The old **netstandard2.0** packages are no longer produced from this tree.

## Commands

From the repo root:

```sh
cd csharp
dotnet build Evoq.Surfdude.sln
dotnet test tests/Evoq.Surfdude.Tests/Evoq.Surfdude.Tests.csproj
dotnet run --project tests/Evoq.Surfdude.ConsoleApp   # optional sample against Apiary mock URL
```

Release packages:

```sh
dotnet pack src/Evoq.Surfdude/Evoq.Surfdude.csproj -c Release
dotnet pack src/Evoq.Surfdude.SimpleJson/Evoq.Surfdude.SimpleJson.csproj -c Release
```

Package ids on NuGet: **`Surfdude`**, **`Surfdude.SimpleJson.Client`**, **`Surfdude.AgentTrader.Client`**. Version tracks repo [`VERSION`](../VERSION) via [`Directory.Build.props`](Directory.Build.props). **GitHub tag releases do not push to NuGet** — build with [`../scripts/tag-release/build-local.sh`](../scripts/tag-release/build-local.sh) or `dotnet pack`, then upload manually. See [../docs/releasing.md#net--nuget-manual](../docs/releasing.md#net--nuget-manual). Legacy script: [build/Publish-ToNuGet.ps1](build/Publish-ToNuGet.ps1).

## Layout

| Path | Role |
|------|------|
| [src/Evoq.Surfdude/](src/Evoq.Surfdude/) | Core abstractions, HTTP steps, `SurfContext`, wave builder |
| [src/Evoq.Surfdude.SimpleJson/](src/Evoq.Surfdude.SimpleJson/) | **simple** dialect, `Surf`, and `Surfer` |
| [tests/Evoq.Surfdude.Tests/](tests/Evoq.Surfdude.Tests/) | xUnit tests (injectable `HttpClient`, per-wave context) |
| [tests/Evoq.Surfdude.ConsoleApp/](tests/Evoq.Surfdude.ConsoleApp/) | Apiary-style sample ride |

Shared hypermedia rules, retries, credentials, prefilled **value**, and [stateless multi-step journals](../docs/model.md#stateless-journeys-journal-on-the-wire): [docs/model.md](../docs/model.md).

## Using the client

**One API, many journeys.** Build a `Surfer` once (shared `HttpClient`, logger). Each `Wave()` gets its own `SurfContext`, so expected status codes and `ThrowOnError` do not leak between journeys.

```csharp
var surfer = Surfer.For("https://api.example.com/")
    .WithHttpClient(httpClient)   // retries, auth, timeouts live in the handler pipeline
    .WithLoggerFactory(loggerFactory);

await surfer.Wave()
    .FromRoot()
    .Then("registrations")
    .ThenItem(0)
    .ThenSubmit("update-contact-details", new { email = "mike@beans.com" })
    .GoAsync();
```

`Surf.Wave(rootUri)` and `Surf.Wave(SurfContext)` remain for older call sites. They use the same shared default `HttpClient` as `Surfer.For` when you do not pass a client.

Surfdude does **not** retry inside the library. For production, attach Polly or `Microsoft.Extensions.Http.Resilience` to the `HttpClient` you pass in. See [Driving a real server](../README.md#driving-a-real-server) in the root README.

### Signing in again

A surfer cannot add a handler to a client it is given, so the handler goes in the client's pipeline:

```csharp
HttpClient client = null;
var handler = new SignInHandler(async (unauthorized, ct) =>
{
    // unauthorized is the 401 that showed the need. Sign in however the API says, through the same client.
    var response = await client.PostAsync(signInUrl, credentialContent, ct);
    return await response.Content.ReadAsStringAsync(ct);   // the access token
}) { InnerHandler = new HttpClientHandler() };
client = new HttpClient(handler);

var surfer = Surfer.For(root).WithHttpClient(client);
```

The handler sends its token as `Authorization: Bearer`. When a request is refused with a 401 it calls your delegate, keeps the token, and sends the same request again once, with the body intact. Requests that fail together share one sign-in. A wave that lists 401 in `ExpectedStatusCodes` is testing the refusal and is left alone, and the requests the delegate sends cannot call it again. A delegate that throws, or returns no token, fails the step with `SignInFailedException`. The request body is held in memory so it can be sent again. Nothing signs in before the first request: the first 401 is what asks for it, in every language.

### AgentTrader

`Surfdude.AgentTrader.Client` reads the AgentTrader envelope (`_links`, `_actions` with `fields`, `data` items with their own controls) and builds the request each control describes: server-prefilled values first, then yours, checked against the control's inputs; JSON body for POST, PUT and PATCH, query string for GET, `If-Match` from `ifMatch`, a dotted name nested in the body.

```csharp
var surfer = AgentTraderSurfer.For("https://agenttrader.net/").WithHttpClient(client);
await surfer.Wave().FromRoot().ThenSubmit("search-listings", new { make = "MINI" }).GoAsync();
```

Every key on a link or action that the model has no member for, such as `requires`, `price`, `title` and `type`, is kept in `IHypertextControl.ControlData` as the JSON text the server sent. `AgentTraderControlData` names the common ones. Nothing in the core acts on them: a 401 or 402 is what tells the client a session or payment is needed (see `SignInHandler`). Status, notes, paging and problems have nowhere to go in the C# model yet, so the shared conformance cases check controls only.

Dialect contract and fixtures: [docs/dialects.md](../docs/dialects.md). Prefilled **`value`** on simple `form` entries is merged on submit like AgentTrader `fields[].value` — see [Form values](../docs/model.md#form-values-for-api-writers-and-for-journeys).

The **simple** dialect expects documents like [fixtures/simple/root.json](../fixtures/simple/root.json): top-level `links` (optional `form` on each link) and `items` for list members. Example with a template and optional query inputs:

```json
{
  "links": [{
    "rel": "registration-finder",
    "href": "https://example.test/registrations/{status}",
    "form": [
      {"name": "status", "options": ["active", "inactive", "any"]},
      {"name": "email"}
    ]
  }]
}
```

The journey supplies `status` and `email` when needed; the client expands `{status}` in the URL. To prefill a field server-side, set `"value"` on that form entry.
