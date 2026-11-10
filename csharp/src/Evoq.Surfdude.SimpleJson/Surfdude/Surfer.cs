namespace Evoq.Surfdude
{
    using Evoq.Surfdude.Hypertext.Http;
    using Microsoft.Extensions.Logging;
    using System;
    using System.Collections.Generic;
    using System.Net.Http;

    /// <summary>
    /// A long-lived client for one API. It holds what every journey shares, the <see cref="HttpClient"/>
    /// and the logger, and starts a fresh wave for each journey. Build it once and keep it.
    /// </summary>
    /// <remarks>
    /// A surfer does not retry. To retry, put a handler in the <see cref="HttpClient"/> that you give it,
    /// for example a Polly policy from Microsoft.Extensions.Http.Resilience. The same goes for timeouts,
    /// authentication and proxies. What belongs to one journey, such as the statuses it expects, is set on
    /// that wave with <see cref="Wave(Action{SurfContext})"/> and never leaks into another.
    /// A surfer is immutable: every With method returns a new one.
    /// </remarks>
    public sealed class Surfer
    {
        private readonly HttpClient httpClient;
        private readonly ILoggerFactory loggerFactory;
        private readonly IReadOnlyList<Challenge> challenges;

        private Surfer(string rootUri, HttpClient httpClient, ILoggerFactory loggerFactory, IReadOnlyList<Challenge> challenges)
        {
            this.RootUri = rootUri;
            this.httpClient = httpClient;
            this.loggerFactory = loggerFactory;
            this.challenges = challenges ?? Array.Empty<Challenge>();
        }

        //

        public string RootUri { get; }

        //

        /// <summary>
        /// Starts a surfer for the API at <paramref name="rootUri"/>, using the shared default client.
        /// </summary>
        public static Surfer For(string rootUri)
        {
            if (string.IsNullOrWhiteSpace(rootUri))
            {
                throw new ArgumentNullOrWhitespaceException(nameof(rootUri));
            }

            return new Surfer(rootUri, httpClient: null, loggerFactory: null, challenges: null);
        }

        /// <summary>
        /// Returns a surfer that sends every request through <paramref name="httpClient"/>. The surfer does
        /// not own the client, so it never disposes it.
        /// </summary>
        public Surfer WithHttpClient(HttpClient httpClient)
        {
            return new Surfer(
                this.RootUri,
                httpClient ?? throw new ArgumentNullException(nameof(httpClient)),
                this.loggerFactory,
                this.challenges);
        }

        public Surfer WithLoggerFactory(ILoggerFactory loggerFactory)
        {
            return new Surfer(this.RootUri, this.httpClient, loggerFactory, this.challenges);
        }

        /// <summary>
        /// Returns a surfer that tries these challenge handlers on unexpected statuses before failing a step.
        /// </summary>
        public Surfer WithChallenges(params Challenge[] challenges)
        {
            return new Surfer(this.RootUri, this.httpClient, this.loggerFactory, challenges ?? Array.Empty<Challenge>());
        }

        /// <summary>
        /// Starts a new journey.
        /// </summary>
        public IWaveStart Wave()
        {
            return this.Wave(configure: null);
        }

        /// <summary>
        /// Starts a new journey, after letting <paramref name="configure"/> set what is particular to it.
        /// Each wave gets its own context, so nothing set here reaches another wave.
        /// </summary>
        public IWaveStart Wave(Action<SurfContext> configure)
        {
            var context = new SurfContext(this.RootUri, this.loggerFactory)
            {
                Challenges = new List<Challenge>(this.challenges),
            };

            configure?.Invoke(context);

            return new Surf(context, this.httpClient);
        }
    }
}
