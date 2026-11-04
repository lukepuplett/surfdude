namespace Evoq.Surfdude.Tests
{
    using Evoq.Surfdude.Hypertext;
    using Evoq.Surfdude.Hypertext.AgentTrader;
    using Evoq.Surfdude.Hypertext.Http;
    using System.Net;
    using System.Net.Http;
    using System.Text;
    using System.Text.Json;

    /// <summary>
    /// The fixtures are shared with Go and Python (/fixtures/agenttrader). C# can hold the controls, so the
    /// controls are what is checked here. Status, notes, paging and problems have nowhere to go in the C#
    /// model yet, so those parts of cases.json are not checked.
    /// </summary>
    public class AgentTraderFixtureTests
    {
        private const string Base = "https://example.test/";

        public static IEnumerable<object[]> Cases()
        {
            using var cases = JsonDocument.Parse(File.ReadAllText(Path.Combine(Fixtures.Dir, "cases.json")));

            return cases.RootElement.GetProperty("cases").EnumerateArray()
                .Select(c => new object[] { c.GetProperty("name").GetString() })
                .ToArray();
        }

        [Theory]
        [MemberData(nameof(Cases))]
        public async Task AgentTrader__read__when_given_a_shared_fixture__then_the_controls_match_the_expected_model(string name)
        {
            using var cases = JsonDocument.Parse(File.ReadAllText(Path.Combine(Fixtures.Dir, "cases.json")));
            var c = cases.RootElement.GetProperty("cases").EnumerateArray().Single(x => x.GetProperty("name").GetString() == name);
            var response = Fixtures.Response(c.GetProperty("file").GetString(), (HttpStatusCode)c.GetProperty("status").GetInt32());

            var resource = (AgentTraderResource)await new AgentTraderResourceFormatter().ReadAsResourceAsync(response);

            var expect = c.GetProperty("expect");
            if (expect.TryGetProperty("rels", out var rels))
            {
                Assert.Equal(rels.EnumerateArray().Select(r => r.GetString()), resource.Rels);
            }

            if (expect.TryGetProperty("controls", out var controls))
            {
                CheckControls(resource, controls);
            }

            if (expect.TryGetProperty("items", out var items))
            {
                Assert.Equal(items.GetArrayLength(), resource.Items.Count);

                int i = 0;
                foreach (var item in items.EnumerateArray())
                {
                    if (item.TryGetProperty("rels", out var itemRels))
                    {
                        Assert.Equal(itemRels.EnumerateArray().Select(r => r.GetString()), resource.Items[i].Rels);
                    }

                    if (item.TryGetProperty("controls", out var itemControls))
                    {
                        CheckControls(resource.Items[i], itemControls);
                    }

                    i++;
                }
            }
        }

        private static void CheckControls(IHypertextControls holder, JsonElement expected)
        {
            foreach (var want in expected.EnumerateObject())
            {
                var control = holder.GetControl(want.Name);
                var w = want.Value;
                string at = $"control '{want.Name}'";

                Assert.True(w.GetProperty("form").GetBoolean() == (control.Inputs != null), $"{at}: form");
                Assert.Equal(w.GetProperty("method").GetString(), control.DetermineHttpMethod().Method);

                if (w.TryGetProperty("inputs", out var inputs))
                {
                    Assert.Equal(inputs.EnumerateArray().Select(x => x.GetString()), control.Inputs.Select(x => x.Name));
                }

                if (w.TryGetProperty("required", out var required))
                {
                    Assert.Equal(required.EnumerateArray().Select(x => x.GetString()), control.Inputs.GetRequiredHypertextControls().Select(x => x.Name));
                }

                if (w.TryGetProperty("extensions", out var extensions))
                {
                    foreach (var extension in extensions.EnumerateObject())
                    {
                        Assert.Equal(Normalised(extension.Value.GetRawText()), Normalised(Data(control, extension.Name)));
                    }
                }

                if (w.TryGetProperty("ifMatch", out var ifMatch))
                {
                    Assert.True(control.TryParseIfMatch(out string actual), $"{at}: ifMatch present");
                    Assert.Equal(ifMatch.GetString(), actual);
                }

                if (w.TryGetProperty("inputValues", out var values))
                {
                    foreach (var v in values.EnumerateObject())
                    {
                        string expectedText = v.Value.ValueKind == JsonValueKind.String ? v.Value.GetString() : v.Value.GetRawText();
                        Assert.Equal(expectedText, control.Inputs.Single(x => x.Name == v.Name).Value);
                    }
                }
            }
        }

        // The same JSON whatever its spacing.
        private static string Normalised(string json) =>
            json == null ? null : JsonSerializer.Serialize(JsonDocument.Parse(json).RootElement);

        private static string Data(IHypertextControl control, string key) =>
            control.ControlData.FirstOrDefault(d => d.Key == key).Value;
    }

    public class AgentTraderSurferTests
    {
        private const string Root = "https://example.test/";

        private sealed class Server : HttpMessageHandler
        {
            public List<HttpRequestMessage> Requests { get; } = new List<HttpRequestMessage>();

            public List<string> Bodies { get; } = new List<string>();

            protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
            {
                this.Requests.Add(request);
                this.Bodies.Add(request.Content == null ? "" : await request.Content.ReadAsStringAsync(cancellationToken));

                return request.RequestUri.AbsolutePath switch
                {
                    "/" => Fixtures.Response("home.json"),
                    "/listings/lst_seed01" => Fixtures.Response("listing-show.json"),
                    "/register" or "/auth/token" => Fixtures.Response("home.json"),
                    _ => Fixtures.Response("listings-search.json"),
                };
            }
        }

        private static (AgentTraderSurfer surfer, Server server) Arrange()
        {
            var server = new Server();

            return (AgentTraderSurfer.For(Root).WithHttpClient(new HttpClient(server)), server);
        }

        [Fact]
        public async Task AgentTrader__Wave__when_a_control_has_requires_and_price__then_they_travel_as_json_in_the_control_data()
        {
            var resource = (AgentTraderResource)await new AgentTraderResourceFormatter().ReadAsResourceAsync(Fixtures.Response("listing-show.json"));

            var offer = resource.GetControl("make-offer");

            string requires = offer.ControlData.Single(d => d.Key == AgentTraderControlData.Requires).Value;
            string price = offer.ControlData.Single(d => d.Key == AgentTraderControlData.Price).Value;
            Assert.Equal("[\"agent-session\"]", requires);
            Assert.Equal("0.50", JsonDocument.Parse(price).RootElement.GetProperty("amount").GetString());
        }

        [Fact]
        public async Task AgentTrader__Wave__when_a_control_has_neither_requires_nor_price__then_the_control_data_has_no_such_keys()
        {
            var resource = (AgentTraderResource)await new AgentTraderResourceFormatter().ReadAsResourceAsync(Fixtures.Response("home.json"));

            var self = resource.GetControl("self");

            Assert.DoesNotContain(self.ControlData, d => d.Key == AgentTraderControlData.Requires || d.Key == AgentTraderControlData.Price);
        }

        [Fact]
        public async Task AgentTrader__Wave__when_an_href_is_relative__then_it_is_followed_against_the_page_it_came_from()
        {
            var (surfer, server) = Arrange();

            await surfer.Wave().FromRoot().Then("search-listings").GoAsync();

            Assert.Equal("https://example.test/listings", server.Requests[1].RequestUri.GetLeftPart(UriPartial.Path));
        }

        [Fact]
        public async Task AgentTrader__Wave__when_a_get_form_is_followed__then_the_inputs_go_in_the_query()
        {
            var (surfer, server) = Arrange();

            await surfer.Wave().FromRoot().ThenSubmit("search-listings", new { make = "MINI", take = 5 }).GoAsync();

            var query = server.Requests[1].RequestUri.Query;
            Assert.Contains("make=MINI", query);
            Assert.Contains("take=5", query);
        }

        [Fact]
        public async Task AgentTrader__Wave__when_a_form_is_submitted__then_the_body_is_json_of_the_supplied_values()
        {
            var (surfer, server) = Arrange();

            await surfer.Wave().FromRoot().ThenSubmit("register", new { email = "a@b.test", hashcash = "1:abc" }).GoAsync();

            Assert.Equal(HttpMethod.Post, server.Requests[1].Method);
            var body = JsonDocument.Parse(server.Bodies[1]).RootElement;
            Assert.Equal("a@b.test", body.GetProperty("email").GetString());
            Assert.Equal("1:abc", body.GetProperty("hashcash").GetString());
            Assert.Equal("application/json", server.Requests[1].Content.Headers.ContentType.MediaType);
        }

        [Fact]
        public async Task AgentTrader__Wave__when_the_server_prefilled_a_value_with_a_number_schema__then_the_body_carries_a_number_and_the_if_match_header()
        {
            var (surfer, server) = Arrange();

            await surfer.Wave().From("https://example.test/listings/lst_seed01").ThenSubmit("change-price", new { }).GoAsync();

            Assert.Equal("PATCH", server.Requests[1].Method.Method);
            Assert.Equal(3000, JsonDocument.Parse(server.Bodies[1]).RootElement.GetProperty("askingPriceGbp").GetInt32());
            Assert.Equal("v3", server.Requests[1].Headers.GetValues("If-Match").Single());
        }

        [Fact]
        public async Task AgentTrader__Wave__when_the_journey_supplies_an_unknown_input__then_it_is_refused_before_sending()
        {
            var (surfer, server) = Arrange();

            await Assert.ThrowsAsync<UnexpectedInputsException>(
                () => surfer.Wave().FromRoot().ThenSubmit("register", new { email = "a@b.test", hashcash = "x", nope = "y" }).GoAsync());

            Assert.Single(server.Requests);
        }

        [Fact]
        public async Task AgentTrader__Wave__when_a_required_input_is_missing__then_it_is_refused_before_sending()
        {
            var (surfer, server) = Arrange();

            await Assert.ThrowsAsync<MissingInputException>(
                () => surfer.Wave().FromRoot().ThenSubmit("register", new { email = "a@b.test" }).GoAsync());

            Assert.Single(server.Requests);
        }

        [Fact]
        public void AgentTrader__BuildRequest__when_a_name_is_dotted__then_the_body_nests()
        {
            var control = new AgentTraderControl
            {
                Rel = "x",
                HRef = "https://example.test/x",
                Inputs = new IHypertextInputControl[]
                {
                    new AgentTraderInput { Name = "location.lat", IsOptional = true, SchemaType = "number" },
                    new AgentTraderInput { Name = "location.lon", IsOptional = true, SchemaType = "number" },
                },
                Method = "POST",
                ControlData = Array.Empty<KeyValuePair<string, string>>(),
            };

            var request = new AgentTraderResourceFormatter().BuildRequest(
                new Dictionary<string, string> { ["location.lat"] = "51.5", ["location.lon"] = "-0.1" }, control);

            var body = JsonDocument.Parse(request.Content.ReadAsStringAsync().Result).RootElement.GetProperty("location");
            Assert.Equal(51.5, body.GetProperty("lat").GetDouble());
            Assert.Equal(-0.1, body.GetProperty("lon").GetDouble());
        }

        [Fact]
        public async Task AgentTrader__Wave__when_a_401_comes_back__then_the_sign_in_handler_signs_in_and_the_wave_carries_on()
        {
            int signIns = 0;
            var api = new SessionServer();
            HttpClient client = null;
            var handler = new SignInHandler(async (unauthorized, ct) =>
            {
                signIns++;
                var response = await client.PostAsync("https://example.test/auth/token", new StringContent("{}"), ct);

                return await response.Content.ReadAsStringAsync(ct);
            }, "stale") { InnerHandler = api };
            client = new HttpClient(handler);

            await AgentTraderSurfer.For(Root).WithHttpClient(client).Wave().FromRoot().Then("new-listing").GoAsync();

            Assert.Equal(1, signIns);
            Assert.Equal(new[] { "Bearer stale", "Bearer T1" }, api.SeenAuth);
        }

        private sealed class SessionServer : HttpMessageHandler
        {
            public List<string> SeenAuth { get; } = new List<string>();

            protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
            {
                string path = request.RequestUri.AbsolutePath;

                if (path == "/auth/token")
                {
                    return Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK) { Content = new StringContent("T1") });
                }

                if (path == "/")
                {
                    return Task.FromResult(Fixtures.Response("home.json"));
                }

                this.SeenAuth.Add(request.Headers.Authorization?.ToString());

                return Task.FromResult(request.Headers.Authorization?.Parameter == "T1"
                    ? Fixtures.Response("listings-search.json")
                    : Fixtures.Response("listing-422.json", HttpStatusCode.Unauthorized));
            }
        }
    }

    internal static class Fixtures
    {
        public static string Dir { get; } = Find();

        public static HttpResponseMessage Response(string file, HttpStatusCode status = HttpStatusCode.OK)
        {
            var response = new HttpResponseMessage(status)
            {
                Content = new StringContent(File.ReadAllText(Path.Combine(Dir, file)), Encoding.UTF8, "application/json"),
                RequestMessage = new HttpRequestMessage(HttpMethod.Get, "https://example.test/"),
            };
            response.Headers.TryAddWithoutValidation("Link", "<https://agenttrader.net/profiles/envelope/v1>; rel=\"profile\"");

            return response;
        }

        private static string Find()
        {
            for (var dir = new DirectoryInfo(AppContext.BaseDirectory); dir != null; dir = dir.Parent)
            {
                string candidate = Path.Combine(dir.FullName, "fixtures", "agenttrader");
                if (File.Exists(Path.Combine(candidate, "cases.json")))
                {
                    return candidate;
                }
            }

            throw new DirectoryNotFoundException("Unable to find fixtures/agenttrader. It sits at the root of the repository.");
        }
    }
}
