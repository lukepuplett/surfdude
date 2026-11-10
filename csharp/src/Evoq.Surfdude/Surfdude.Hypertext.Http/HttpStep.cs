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

        protected ReplayableRequest LastReplayable { get; private set; }

        //

        public async Task ExecuteAsync(IStep previous, CancellationToken cancellationToken)
        {
            this.Response = await this.ExecuteStepRequestAsync((HttpStep)previous, cancellationToken);

            var challenged = false;
            while (!this.IsExpectedStatus(this.Response.StatusCode))
            {
                if (!challenged && this.LastReplayable != null)
                {
                    var body = await this.Response.Content.ReadAsByteArrayAsync(cancellationToken).ConfigureAwait(false);
                    IHypertextResource refused;
                    try
                    {
                        refused = await this.StepContext.ResourceFormatter.ReadAsResourceAsync(this.Response).ConfigureAwait(false);
                    }
                    catch
                    {
                        refused = null;
                    }

                    if (refused != null)
                    {
                        var retry = await ChallengeRunner.TryAsync(
                            this.StepContext,
                            refused,
                            this.Response,
                            body,
                            cancellationToken).ConfigureAwait(false);
                        if (retry)
                        {
                            challenged = true;
                            this.Response?.Dispose();
                            this.Response = await this.SendReplayableAsync(
                                this.LastReplayable.CreateAgain(),
                                cancellationToken).ConfigureAwait(false);
                            continue;
                        }
                    }
                }

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

        protected async Task<HttpResponseMessage> SendReplayableAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            this.LastReplayable = await ReplayableRequest.CaptureAsync(request, cancellationToken).ConfigureAwait(false);
            return await this.SendAsync(request, cancellationToken).ConfigureAwait(false);
        }

        protected Task<HttpResponseMessage> GetAsync(string uri, CancellationToken cancellationToken)
        {
            return this.SendReplayableAsync(new HttpRequestMessage(HttpMethod.Get, new Uri(uri, UriKind.RelativeOrAbsolute)), cancellationToken);
        }

        internal abstract Task<HttpResponseMessage> ExecuteStepRequestAsync(HttpStep previous, CancellationToken cancellationToken);
    }
}