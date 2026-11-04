namespace Evoq.Surfdude.Hypertext.AgentTrader
{
    using Evoq.Surfdude.Hypertext.Http;
    using System;
    using System.Collections.Generic;
    using System.Net.Http;
    using System.Text.Json;
    using System.Threading.Tasks;

    /// <summary>
    /// Reads the AgentTrader response envelope and builds the requests its controls describe.
    /// </summary>
    internal sealed class AgentTraderResourceFormatter : IHypertextResourceFormatter
    {
        private static readonly JsonSerializerOptions ModelOptions = new JsonSerializerOptions { PropertyNameCaseInsensitive = true };

        public async Task<IHypertextResource> ReadAsResourceAsync(HttpResponseMessage httpResponse)
        {
            if (httpResponse == null)
            {
                throw new ArgumentNullException(nameof(httpResponse));
            }

            using (var document = await ParseAsync(httpResponse))
            {
                return AgentTraderDocumentReader.Read(document.RootElement, httpResponse.RequestMessage?.RequestUri);
            }
        }

        public async Task<TModel> ReadAsModelAsync<TModel>(HttpResponseMessage httpResponse)
        {
            if (httpResponse == null)
            {
                throw new ArgumentNullException(nameof(httpResponse));
            }

            using (var document = await ParseAsync(httpResponse))
            {
                if (!document.RootElement.TryGetProperty("data", out var data) || data.ValueKind != JsonValueKind.Array)
                {
                    throw new InvalidOperationException("Unable to read data. The response carried none.");
                }

                if (typeof(TModel).IsArray)
                {
                    return data.Deserialize<TModel>(ModelOptions);
                }

                if (data.GetArrayLength() == 0)
                {
                    throw new InvalidOperationException("Unable to read data. The response carried none.");
                }

                return data[0].Deserialize<TModel>(ModelOptions);
            }
        }

        public HttpRequestMessage BuildRequest(IDictionary<string, string> sendPairs, IHypertextControl hypertextControl)
        {
            return AgentTraderRequestBuilder.Build(sendPairs, hypertextControl);
        }

        //

        private static async Task<JsonDocument> ParseAsync(HttpResponseMessage httpResponse)
        {
            var bytes = await httpResponse.Content.ReadAsByteArrayAsync();

            return JsonDocument.Parse(bytes);
        }
    }
}
