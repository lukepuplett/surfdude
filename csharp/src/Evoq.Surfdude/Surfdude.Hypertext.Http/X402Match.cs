namespace Evoq.Surfdude.Hypertext.Http
{
    using System.Net;
    using System.Net.Http.Headers;
    using System.Text.Json;

    /// <summary>
    /// x402 vs AgentTrader envelope discrimination for challenge matchers.
    /// </summary>
    public static class X402Match
    {
        public static bool IsX402Body(byte[] body)
        {
            if (body == null || body.Length == 0)
            {
                return false;
            }

            try
            {
                using var document = JsonDocument.Parse(body);
                return document.RootElement.TryGetProperty("x402Version", out var v)
                    && v.ValueKind == JsonValueKind.Number;
            }
            catch (JsonException)
            {
                return false;
            }
        }

        public static bool MatchX402Payment(HttpStatusCode status, HttpResponseHeaders headers, byte[] body)
        {
            return (int)status == 402 && IsX402Body(body);
        }
    }
}
