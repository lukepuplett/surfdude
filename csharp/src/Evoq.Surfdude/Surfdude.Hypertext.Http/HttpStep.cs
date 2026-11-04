namespace Evoq.Surfdude.Hypertext.Http
{
    using Evoq.Surfdude.Hypertext;
    using System;
    using System.Collections.Generic;
    using System.Linq;
    using System.Net;
    using System.Net.Http;
    using System.Threading;
    using System.Threading.Tasks;

    public abstract class HttpStep : IStep
    {
        public HttpStep(HttpStepContext stepContext)
        {
            this.StepContext = stepContext ?? throw new ArgumentNullException(nameof(stepContext));
        }

        //

        public IHypertextResource Resource { get; private set; }

        public string Name => this.GetType().Name;

        //

        public HttpStepContext StepContext { get; }

        protected internal HttpResponseMessage Response { get; set; }

        //

        public async Task ExecuteAsync(IStep previous, CancellationToken cancellationToken)
        {
            this.Response = await this.ExecuteStepRequestAsync((HttpStep)previous, cancellationToken);

            if (!this.IsExpectedStatus(this.Response.StatusCode))
            {
                try
                {
                    this.Response.EnsureSuccessStatusCode();

                    throw new UnexpectedHttpResponseException(
                        $"The HTTP request failed with status {(int)this.Response.StatusCode} {this.Response.StatusCode}.");
                }
                catch (HttpRequestException httpRequestException)
                {
                    throw new UnexpectedHttpResponseException(
                        $"The HTTP request failed with status {(int)this.Response.StatusCode} {this.Response.StatusCode}. See inner exception.", httpRequestException);
                }
            }

            this.Resource = await this.StepContext.ResourceFormatter.ReadAsResourceAsync(this.Response);
        }

        private bool IsExpectedStatus(HttpStatusCode statusCode)
        {
            return this.StepContext.SurfContext.ExpectedStatusCodes?.Contains((int)statusCode) ?? false;
        }

        /// <summary>
        /// Sends a request through the client, telling the sign-in handler, if there is one, when this wave
        /// expects a 401 so it leaves the refusal alone.
        /// </summary>
        protected Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            if (this.StepContext.SurfContext.ExpectedStatusCodes?.Contains((int)HttpStatusCode.Unauthorized) ?? false)
            {
                request.Options.Set(SignInHandler.ExpectsUnauthorized, true);
            }

            return this.StepContext.HttpClient.SendAsync(request, cancellationToken);
        }

        protected Task<HttpResponseMessage> GetAsync(string uri, CancellationToken cancellationToken)
        {
            return this.SendAsync(new HttpRequestMessage(HttpMethod.Get, new Uri(uri, UriKind.RelativeOrAbsolute)), cancellationToken);
        }

        internal abstract Task<HttpResponseMessage> ExecuteStepRequestAsync(HttpStep previous, CancellationToken cancellationToken);
    }
}