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
    /// Challenge handlers: listing-weeks 402 triggers sandbox credit and retries publish (Go challenge_test.go).
    /// </summary>
    public class ChallengeHandlerTests
    {
        private const string Profile = "<https://agenttrader.net/profiles/envelope/v1>; rel=\"profile\"";

        [Fact]
        public async Task WithChallenges__when_listing_weeks_is_missing__then_credit_runs_and_publish_retries()
        {
            var api = new ListingWeeksServer();
            var client = new HttpClient(api);

            var creditWeeks = new Challenge(
                "listing-weeks",
                AgentTraderMatch.MatchCapabilityGap(HttpStatusCode.PaymentRequired, "listing-weeks"),
                async (cc, ct) =>
                {
                    if (cc.Page.GetControl("sandbox-listing-advert-credit") == null)
                    {
                        return false;
                    }

                    var req = cc.StepContext.ResourceFormatter.BuildRequest(
                        new SendDictionary(new { weeks = 4 }),
                        cc.Page.GetControl("sandbox-listing-advert-credit"));
                    using var resp = await cc.StepContext.HttpClient.SendAsync(req, ct).ConfigureAwait(false);
                    return resp.IsSuccessStatusCode;
                });

            await AgentTraderSurfer.For("http://test/")
                .WithHttpClient(client)
                .WithChallenges(creditWeeks)
                .Wave()
                .FromRoot()
                .ThenSubmit("publish", new { weeks = 2 })
                .GoAsync();

            Assert.Equal(2, api.PublishHits);
            Assert.True(api.Credited);
        }

        [Fact]
        public void MatchCapabilityGap__when_body_is_x402__then_it_does_not_match()
        {
            var matcher = AgentTraderMatch.MatchCapabilityGap(HttpStatusCode.PaymentRequired, "listing-weeks");
            var body = Encoding.UTF8.GetBytes("{\"x402Version\":1,\"accepts\":[{\"scheme\":\"exact\"}]}");

            Assert.False(matcher(HttpStatusCode.PaymentRequired, null, body));
        }

        private sealed class ListingWeeksServer : HttpMessageHandler
        {
            private readonly object gate = new object();

            public int PublishHits { get; private set; }

            public bool Credited { get; private set; }

            protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
            {
                string path = request.RequestUri.AbsolutePath;

                if (request.Method == HttpMethod.Get && path == "/")
                {
                    return Task.FromResult(Json(
                        HttpStatusCode.OK,
                        """
                        {
                          "status": 200,
                          "message": "OK",
                          "data": [{"name": "test"}],
                          "_actions": [{
                            "rel": "publish",
                            "action": "/listings/lst_1/publish",
                            "method": "POST",
                            "enctype": "application/json",
                            "fields": [{"name": "weeks", "required": true, "schema": {"type": "integer"}}]
                          }]
                        }
                        """));
                }

                if (request.Method == HttpMethod.Post && path == "/sandbox/listing-advert-weeks")
                {
                    lock (this.gate)
                    {
                        this.Credited = true;
                    }

                    return Task.FromResult(Json(HttpStatusCode.OK, """{"status":200,"message":"OK","data":[{"balance":4}]}"""));
                }

                if (request.Method == HttpMethod.Post && path == "/listings/lst_1/publish")
                {
                    lock (this.gate)
                    {
                        this.PublishHits++;
                        bool first = this.PublishHits == 1;
                        if (first && !this.Credited)
                        {
                            return Task.FromResult(Json(
                                HttpStatusCode.PaymentRequired,
                                """
                                {
                                  "status": 402,
                                  "message": "Unable to publish the listing. Short of weeks.",
                                  "missing": "listing-weeks",
                                  "data": [{"field": "weeks", "message": "need more"}],
                                  "_actions": [{
                                    "rel": "sandbox-listing-advert-credit",
                                    "action": "/sandbox/listing-advert-weeks",
                                    "method": "POST",
                                    "fields": [{"name": "weeks", "required": true}]
                                  }]
                                }
                                """));
                        }
                    }

                    return Task.FromResult(Json(HttpStatusCode.OK, """{"status":200,"message":"OK","data":[{"state":"live"}]}"""));
                }

                return Task.FromResult(new HttpResponseMessage(HttpStatusCode.NotFound));
            }

            private static HttpResponseMessage Json(HttpStatusCode status, string json)
            {
                var response = new HttpResponseMessage(status)
                {
                    Content = new StringContent(json, Encoding.UTF8, "application/json"),
                };
                response.Headers.TryAddWithoutValidation("Link", Profile);
                return response;
            }
        }
    }
}
