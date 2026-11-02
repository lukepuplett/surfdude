namespace Evoq.Surfdude.Tests
{
    using Evoq.Surfdude.Hypertext;
    using Evoq.Surfdude.Hypertext.Http;
    using Microsoft.Extensions.Logging;
    using System.Net;
    using System.Text;

    /// <summary>
    /// A fake server. It records the path of every request that reaches it.
    /// </summary>
    internal sealed class FakeApi : HttpMessageHandler
    {
        private readonly Func<HttpRequestMessage, int, HttpResponseMessage> respond;

        public FakeApi(Func<HttpRequestMessage, int, HttpResponseMessage> respond = null)
        {
            this.respond = respond ?? ((request, hit) => Doc(request.RequestUri.AbsolutePath == "/"
                ? "{\"links\":[{\"rel\":\"next\",\"href\":\"https://example.test/next\"}],\"items\":[]}"
                : "{\"links\":[],\"items\":[]}"));
        }

        public List<string> Paths { get; } = new List<string>();

        public static HttpResponseMessage Doc(string json, HttpStatusCode status = HttpStatusCode.OK)
        {
            return new HttpResponseMessage(status)
            {
                Content = new StringContent(json, Encoding.UTF8, "application/simple-hypertext+json")
            };
        }

        protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            string path = request.RequestUri.AbsolutePath;
            this.Paths.Add(path);

            return Task.FromResult(this.respond(request, this.Paths.Count(p => p == path)));
        }
    }

    /// <summary>
    /// The shape a caller is told to use for retries: a handler in the client's pipeline.
    /// </summary>
    internal sealed class RetryOn503Handler : DelegatingHandler
    {
        protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            for (int attempt = 1; ; attempt++)
            {
                var response = await base.SendAsync(request, cancellationToken);

                if (response.StatusCode != HttpStatusCode.ServiceUnavailable || attempt == 3)
                {
                    return response;
                }

                response.Dispose();
            }
        }
    }

    internal sealed class FakeLoggerFactory : ILoggerFactory
    {
        // These tests only check that the factory is passed along, so it never makes a logger.
        public ILogger CreateLogger(string categoryName) => null;

        public void AddProvider(ILoggerProvider provider) { }

        public void Dispose() { }
    }

    internal sealed class UnusedFormatter : IHypertextResourceFormatter
    {
        public Task<TModel> ReadAsModelAsync<TModel>(HttpResponseMessage httpResponse) => throw new NotSupportedException();

        public Task<IHypertextResource> ReadAsResourceAsync(HttpResponseMessage httpResponse) => throw new NotSupportedException();

        public HttpRequestMessage BuildRequest(IDictionary<string, string> sendPairs, IHypertextControl hypertextControl) => throw new NotSupportedException();
    }

    public class SurferTests
    {
        private const string Root = "https://example.test/";

        [Fact]
        public async Task Surfer__Wave__when_given_an_HttpClient__then_the_requests_go_through_it()
        {
            var api = new FakeApi();
            var surfer = Surfer.For(Root).WithHttpClient(new HttpClient(api));

            await surfer.Wave().FromRoot().Then("next").GoAsync();

            Assert.Equal(new[] { "/", "/next" }, api.Paths);
        }

        [Fact]
        public async Task Surfer__Wave__when_two_waves_run__then_they_share_the_one_client()
        {
            var api = new FakeApi();
            var surfer = Surfer.For(Root).WithHttpClient(new HttpClient(api));

            await surfer.Wave().FromRoot().Then("next").GoAsync();
            await surfer.Wave().FromRoot().Then("next").GoAsync();

            Assert.Equal(4, api.Paths.Count);
        }

        [Fact]
        public async Task Surfer__Wave__when_the_client_has_a_retrying_handler__then_a_503_is_retried()
        {
            var api = new FakeApi((request, hit) => request.RequestUri.AbsolutePath == "/next" && hit <= 2
                ? FakeApi.Doc("{}", HttpStatusCode.ServiceUnavailable)
                : FakeApi.Doc(request.RequestUri.AbsolutePath == "/"
                    ? "{\"links\":[{\"rel\":\"next\",\"href\":\"https://example.test/next\"}],\"items\":[]}"
                    : "{\"links\":[],\"items\":[]}"));
            var client = new HttpClient(new RetryOn503Handler { InnerHandler = api });
            var surfer = Surfer.For(Root).WithHttpClient(client);

            await surfer.Wave().FromRoot().Then("next").GoAsync();

            Assert.Equal(3, api.Paths.Count(p => p == "/next"));
        }

        [Fact]
        public async Task Surfer__Wave__when_one_wave_expects_a_404__then_another_wave_still_fails_on_it()
        {
            var api = new FakeApi((request, hit) => request.RequestUri.AbsolutePath == "/next"
                ? FakeApi.Doc("{\"links\":[],\"items\":[]}", HttpStatusCode.NotFound)
                : FakeApi.Doc("{\"links\":[{\"rel\":\"next\",\"href\":\"https://example.test/next\"}],\"items\":[]}"));
            var surfer = Surfer.For(Root).WithHttpClient(new HttpClient(api));

            await surfer.Wave(c => c.ExpectedStatusCodes = new[] { 200, 404 }).FromRoot().Then("next").GoAsync();

            await Assert.ThrowsAsync<UnexpectedHttpResponseException>(
                () => surfer.Wave().FromRoot().Then("next").GoAsync());
        }

        [Fact]
        public void Surfer__Wave__when_configuring__then_the_context_carries_the_root_and_the_logger_factory()
        {
            var loggers = new FakeLoggerFactory();
            SurfContext seen = null;

            Surfer.For(Root).WithLoggerFactory(loggers).Wave(c => seen = c);

            Assert.Equal(Root, seen.RootUri);
            Assert.Same(loggers, seen.LoggerFactory);
        }

        [Fact]
        public void Surfer__WithLoggerFactory__when_called__then_the_original_surfer_is_unchanged()
        {
            var original = Surfer.For(Root);
            SurfContext seen = null;

            var changed = original.WithLoggerFactory(new FakeLoggerFactory());
            original.Wave(c => seen = c);

            Assert.NotSame(original, changed);
            Assert.Null(seen.LoggerFactory);
        }

        [Fact]
        public void Surfer__For__when_the_root_is_blank__then_it_throws()
        {
            Assert.Throws<ArgumentNullOrWhitespaceException>(() => Surfer.For(" "));
        }

        [Fact]
        public void StepFactory__when_no_client_is_given__then_factories_share_one_default_client()
        {
            var context = new SurfContext(Root);

            var first = new StepFactory(context, new UnusedFormatter());
            var second = new StepFactory(context, new UnusedFormatter());

            Assert.Same(first.HttpClient, second.HttpClient);
        }

        [Fact]
        public void StepFactory__when_a_client_is_given__then_it_is_the_one_used()
        {
            var context = new SurfContext(Root);
            var mine = new HttpClient(new FakeApi());

            var factory = new StepFactory(context, new UnusedFormatter(), mine);

            Assert.Same(mine, factory.HttpClient);
        }
    }
}
