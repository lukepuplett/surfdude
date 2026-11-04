namespace Evoq.Surfdude.Hypertext.Http
{
    using System.Collections.Generic;
    using System.Net.Http;
    using System.Threading;
    using System.Threading.Tasks;

    /// <summary>
    /// A copy of a request that can be sent again. An <see cref="HttpRequestMessage"/> can only be sent once,
    /// and its content may be gone after the first send, so the body is kept as bytes before that.
    /// </summary>
    internal sealed class ReplayableRequest
    {
        private readonly HttpRequestMessage original;
        private readonly byte[] body;

        private ReplayableRequest(HttpRequestMessage original, byte[] body)
        {
            this.original = original;
            this.body = body;
        }

        //

        public static async Task<ReplayableRequest> CaptureAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            byte[] body = null;

            if (request.Content != null)
            {
                body = await request.Content.ReadAsByteArrayAsync(cancellationToken);
            }

            return new ReplayableRequest(request, body);
        }

        public HttpRequestMessage CreateAgain()
        {
            var again = new HttpRequestMessage(this.original.Method, this.original.RequestUri)
            {
                Version = this.original.Version,
                VersionPolicy = this.original.VersionPolicy,
            };

            foreach (var header in this.original.Headers)
            {
                again.Headers.TryAddWithoutValidation(header.Key, header.Value);
            }

            // HttpRequestOptions is a dictionary of the keys the options were set with.
            foreach (var option in (IDictionary<string, object>)this.original.Options)
            {
                ((IDictionary<string, object>)again.Options)[option.Key] = option.Value;
            }

            if (this.body != null)
            {
                again.Content = new ByteArrayContent(this.body);

                foreach (var header in this.original.Content.Headers)
                {
                    again.Content.Headers.TryAddWithoutValidation(header.Key, header.Value);
                }
            }

            return again;
        }
    }
}
