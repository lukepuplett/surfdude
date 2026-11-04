namespace Evoq.Surfdude
{
    using Microsoft.Extensions.Logging;
    using System;
    using System.Net.Http;

    /// <summary>
    /// A long-lived client for one API that speaks the AgentTrader envelope. It holds what every journey
    /// shares, the <see cref="HttpClient"/> and the logger, and starts a fresh wave for each journey.
    /// Sessions, retries and timeouts belong in the client's handler pipeline: for signing in, put a
    /// <see cref="Hypertext.Http.SignInHandler"/> in it. A surfer is immutable: every With method returns a new one.
    /// </summary>
    public sealed class AgentTraderSurfer
    {
        private readonly HttpClient httpClient;
        private readonly ILoggerFactory loggerFactory;

        private AgentTraderSurfer(string rootUri, HttpClient httpClient, ILoggerFactory loggerFactory)
        {
            this.RootUri = rootUri;
            this.httpClient = httpClient;
            this.loggerFactory = loggerFactory;
        }

        //

        public string RootUri { get; }

        //

        public static AgentTraderSurfer For(string rootUri)
        {
            if (string.IsNullOrWhiteSpace(rootUri))
            {
                throw new ArgumentNullOrWhitespaceException(nameof(rootUri));
            }

            return new AgentTraderSurfer(rootUri, httpClient: null, loggerFactory: null);
        }

        public AgentTraderSurfer WithHttpClient(HttpClient httpClient)
        {
            return new AgentTraderSurfer(
                this.RootUri,
                httpClient ?? throw new ArgumentNullException(nameof(httpClient)),
                this.loggerFactory);
        }

        public AgentTraderSurfer WithLoggerFactory(ILoggerFactory loggerFactory)
        {
            return new AgentTraderSurfer(this.RootUri, this.httpClient, loggerFactory);
        }

        public IWaveStart Wave()
        {
            return this.Wave(configure: null);
        }

        public IWaveStart Wave(Action<SurfContext> configure)
        {
            var context = new SurfContext(this.RootUri, this.loggerFactory);

            configure?.Invoke(context);

            return new AgentTraderSurf(context, this.httpClient);
        }
    }
}
