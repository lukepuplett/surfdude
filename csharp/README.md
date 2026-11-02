# Surfdude for C#

The original Surfdude client. It drives an API by **rel** names using the **simple** dialect (`application/simple-hypertext+json`). Go and Python in this repo share the same journey ideas and [fixtures](../fixtures/); they ship **agenttrader** as well. C# can plug in another formatter via `IHypertextResourceFormatter` (AgentTrader is still on the roadmap).

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

Publishing to nuget.org is scripted in [build/Publish-ToNuGet.ps1](build/Publish-ToNuGet.ps1).

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
