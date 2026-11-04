namespace Evoq.Surfdude.Tests
{
    using Evoq.Surfdude.Hypertext.Http;
    using System.Net;
    using System.Net.Http;
    using System.Text;

    /// <summary>
    /// A fake API with a session: sign in with the credential "good" to get a token, and the protected route
    /// answers 401 to any other token. These are the C# copies of go/surf/signin_test.go.
    /// </summary>
    internal sealed class AuthApi : HttpMessageHandler
    {
        private const string Home =
            "{\"links\":[" +
            "{\"rel\":\"protected\",\"href\":\"https://example.test/protected\"}," +
            "{\"rel\":\"make-offer\",\"href\":\"https://example.test/offers\",\"method\":\"POST\",\"form\":[{\"name\":\"amount\"}]}" +
            "],\"items\":[]}";

        private readonly object gate = new object();
        private readonly HashSet<string> valid = new HashSet<string>();
        private int issued;

        public int SignIns { get; private set; }

        public bool RefuseNew { get; set; }

        public List<string> SeenAuth { get; } = new List<string>();

        public List<string> OfferBodies { get; } = new List<string>();

        protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            string path = request.RequestUri.AbsolutePath;
            string body = request.Content == null ? "" : await request.Content.ReadAsStringAsync(cancellationToken);

            if (path == "/")
            {
                return FakeApi.Doc(Home);
            }

            if (path == "/auth/token")
            {
                return this.IssueToken(body);
            }

            return this.Protected(request, path, body);
        }

        private HttpResponseMessage IssueToken(string body)
        {
            if (!body.Contains("\"good\""))
            {
                return FakeApi.Doc("{\"links\":[],\"items\":[]}", HttpStatusCode.Unauthorized);
            }

            lock (this.gate)
            {
                this.SignIns++;
                string token = "T" + (++this.issued);
                if (!this.RefuseNew)
                {
                    this.valid.Add(token);
                }

                return new HttpResponseMessage(HttpStatusCode.OK) { Content = new StringContent(token, Encoding.UTF8, "text/plain") };
            }
        }

        private HttpResponseMessage Protected(HttpRequestMessage request, string path, string body)
        {
            string auth = request.Headers.Authorization?.ToString() ?? "";

            lock (this.gate)
            {
                this.SeenAuth.Add(auth);

                if (!this.valid.Contains(request.Headers.Authorization?.Parameter ?? ""))
                {
                    var refused = FakeApi.Doc("{\"links\":[],\"items\":[]}", HttpStatusCode.Unauthorized);
                    refused.Headers.WwwAuthenticate.ParseAdd("Bearer realm=\"test\"");
                    return refused;
                }

                if (path == "/offers")
                {
                    this.OfferBodies.Add(body);
                }
            }

            return FakeApi.Doc("{\"links\":[],\"items\":[]}");
        }
    }

    public class SignInHandlerTests
    {
        private const string Root = "https://example.test/";

        /// <summary>
        /// The kind of delegate a test would write: ask the sign-in route for a token, through the same client.
        /// The request goes through the handler, which is how the no-recursion guard gets exercised.
        /// </summary>
        private static SignInDelegate SignInWith(string credential, HttpClient client, List<string> calls)
        {
            return async (unauthorized, ct) =>
            {
                lock (calls)
                {
                    calls.Add(credential);
                }

                var response = await client.PostAsync(
                    "https://example.test/auth/token",
                    new StringContent($"\"{credential}\"", Encoding.UTF8, "application/json"),
                    ct);

                response.EnsureSuccessStatusCode();

                return await response.Content.ReadAsStringAsync(ct);
            };
        }

        private static (Surfer surfer, AuthApi api, SignInHandler handler, List<string> calls) Arrange(
            string credential = "good", string token = "stale", bool refuseNew = false)
        {
            var api = new AuthApi { RefuseNew = refuseNew };
            var calls = new List<string>();
            HttpClient client = null;
            var handler = new SignInHandler((u, ct) => SignInWith(credential, client, calls)(u, ct), token) { InnerHandler = api };
            client = new HttpClient(handler);

            return (Surfer.For(Root).WithHttpClient(client), api, handler, calls);
        }

        [Fact]
        public async Task SignInHandler__when_a_request_gets_401__then_it_signs_in_and_sends_the_same_request_once_more()
        {
            var (surfer, api, handler, calls) = Arrange();

            await surfer.Wave().FromRoot().Then("protected").GoAsync();

            Assert.Single(calls);
            Assert.Equal(1, api.SignIns);
            Assert.Equal(new[] { "Bearer stale", "Bearer T1" }, api.SeenAuth);
            Assert.Equal("T1", handler.Token);
        }

        [Fact]
        public async Task SignInHandler__when_the_token_was_replaced__then_the_next_request_needs_no_sign_in()
        {
            var (surfer, api, _, _) = Arrange();

            await surfer.Wave().FromRoot().Then("protected").GoAsync();
            await surfer.Wave().FromRoot().Then("protected").GoAsync();

            Assert.Equal(1, api.SignIns);
        }

        [Fact]
        public async Task SignInHandler__when_a_form_is_submitted__then_the_body_arrives_intact_after_the_replay()
        {
            var (surfer, api, _, _) = Arrange();

            await surfer.Wave().FromRoot().ThenSubmit("make-offer", new { amount = "2750" }).GoAsync();

            Assert.Single(api.OfferBodies);
            Assert.Contains("2750", api.OfferBodies[0]);
        }

        [Fact]
        public async Task SignInHandler__when_the_wave_expects_401__then_it_is_left_alone()
        {
            var (surfer, _, _, calls) = Arrange();

            await surfer.Wave(c => c.ExpectedStatusCodes = new[] { 200, 401 }).FromRoot().Then("protected").GoAsync();

            Assert.Empty(calls);
        }

        [Fact]
        public async Task SignInHandler__when_the_new_token_is_refused_too__then_it_gives_up_after_one_try()
        {
            var (surfer, api, _, calls) = Arrange(refuseNew: true);

            await Assert.ThrowsAsync<UnexpectedHttpResponseException>(
                () => surfer.Wave().FromRoot().Then("protected").GoAsync());

            Assert.Single(calls);
            Assert.Equal(2, api.SeenAuth.Count);
        }

        [Fact]
        public async Task SignInHandler__when_signing_in_is_refused__then_the_delegate_is_not_called_again()
        {
            var (surfer, _, _, calls) = Arrange(credential: "wrong");

            var failed = await Assert.ThrowsAsync<SignInFailedException>(
                () => surfer.Wave().FromRoot().Then("protected").GoAsync());

            Assert.Single(calls);
            Assert.IsType<HttpRequestException>(failed.InnerException);
        }

        [Fact]
        public async Task SignInHandler__when_the_delegate_throws__then_the_step_reports_why()
        {
            var boom = new InvalidOperationException("the credential store is locked");
            var api = new AuthApi();
            var handler = new SignInHandler((u, ct) => throw boom, "stale") { InnerHandler = api };
            var surfer = Surfer.For(Root).WithHttpClient(new HttpClient(handler));

            var failed = await Assert.ThrowsAsync<SignInFailedException>(
                () => surfer.Wave().FromRoot().Then("protected").GoAsync());

            Assert.Same(boom, failed.InnerException);
            Assert.Contains("the credential store is locked", failed.Message);
        }

        [Fact]
        public async Task SignInHandler__when_the_delegate_returns_no_token__then_the_step_fails()
        {
            var api = new AuthApi();
            var handler = new SignInHandler((u, ct) => Task.FromResult(""), "stale") { InnerHandler = api };
            var surfer = Surfer.For(Root).WithHttpClient(new HttpClient(handler));

            var failed = await Assert.ThrowsAsync<SignInFailedException>(
                () => surfer.Wave().FromRoot().Then("protected").GoAsync());

            Assert.Contains("no token", failed.Message);
        }

        [Fact]
        public async Task SignInHandler__when_the_delegate_runs__then_it_can_read_the_401_it_was_given()
        {
            var api = new AuthApi();
            HttpStatusCode? seenStatus = null;
            string seenChallenge = null;
            HttpClient client = null;
            var handler = new SignInHandler(async (unauthorized, ct) =>
            {
                seenStatus = unauthorized.StatusCode;
                seenChallenge = unauthorized.Headers.WwwAuthenticate.ToString();
                return await SignInWith("good", client, new List<string>())(unauthorized, ct);
            }, "stale") { InnerHandler = api };
            client = new HttpClient(handler);

            await Surfer.For(Root).WithHttpClient(client).Wave().FromRoot().Then("protected").GoAsync();

            Assert.Equal(HttpStatusCode.Unauthorized, seenStatus);
            Assert.Contains("realm=\"test\"", seenChallenge);
        }

        [Fact]
        public async Task SignInHandler__when_many_requests_fail_at_once__then_they_share_one_sign_in()
        {
            for (int run = 0; run < 10; run++)
            {
                var (surfer, api, _, calls) = Arrange();

                var waves = Enumerable.Range(0, 6)
                    .Select(_ => Task.Run(() => surfer.Wave().FromRoot().Then("protected").GoAsync()))
                    .ToArray();

                await Task.WhenAll(waves);

                Assert.True(
                    api.SignIns == 1 && calls.Count == 1,
                    $"run {run}: {api.SignIns} sign-ins and {calls.Count} delegate calls for 6 concurrent waves, want 1 and 1");
            }
        }

        [Fact]
        public async Task SignInHandler__when_no_token_is_held_and_nothing_is_refused__then_no_sign_in_happens()
        {
            var api = new FakeApi();
            var handler = new SignInHandler((u, ct) => throw new InvalidOperationException("should not run")) { InnerHandler = api };
            var surfer = Surfer.For(Root).WithHttpClient(new HttpClient(handler));

            await surfer.Wave().FromRoot().Then("next").GoAsync();

            Assert.Equal(new[] { "/", "/next" }, api.Paths);
        }
    }
}
