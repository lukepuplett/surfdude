namespace Evoq.Surfdude.Hypertext.AgentTrader
{
    using Evoq.Surfdude.Hypertext.Http;
    using System;
    using System.Linq;
    using System.Net;
    using System.Net.Http.Headers;
    using System.Text.Json;

    /// <summary>
    /// Matchers and probes for AgentTrader envelope refusals (for example capability gaps).
    /// </summary>
    public static class AgentTraderMatch
    {
        public const string ProfileUri = "https://agenttrader.net/profiles/envelope/v1";

        public static bool HasProfileLink(HttpResponseHeaders headers)
        {
            if (headers == null)
            {
                return false;
            }

            foreach (var link in headers.Where(h => h.Key.Equals("Link", StringComparison.OrdinalIgnoreCase)).SelectMany(h => h.Value))
            {
                if (link.Contains("<" + ProfileUri + ">", StringComparison.Ordinal) && link.Contains("rel=\"profile\"", StringComparison.Ordinal))
                {
                    return true;
                }
            }

            return false;
        }

        public static bool TryParseEnvelopeProbe(byte[] body, out int status, out string missing)
        {
            status = 0;
            missing = "";
            if (body == null || body.Length == 0)
            {
                return false;
            }

            try
            {
                using var document = JsonDocument.Parse(body);
                var root = document.RootElement;
                if (!root.TryGetProperty("status", out var statusEl) || statusEl.ValueKind != JsonValueKind.Number)
                {
                    return false;
                }

                if (!root.TryGetProperty("data", out var data) || data.ValueKind != JsonValueKind.Array)
                {
                    return false;
                }

                status = statusEl.GetInt32();
                if (root.TryGetProperty("missing", out var missingEl) && missingEl.ValueKind == JsonValueKind.String)
                {
                    missing = missingEl.GetString();
                }

                return true;
            }
            catch (JsonException)
            {
                return false;
            }
        }

        public static ChallengeMatcher MatchCapabilityGap(HttpStatusCode status, string missing)
        {
            return (got, headers, body) =>
            {
                if (got != status)
                {
                    return false;
                }

                if (X402Match.IsX402Body(body))
                {
                    return false;
                }

                if (!TryParseEnvelopeProbe(body, out var envStatus, out var envMissing) || envMissing != missing)
                {
                    return false;
                }

                return HasProfileLink(headers) || envStatus == (int)got;
            };
        }
    }
}
