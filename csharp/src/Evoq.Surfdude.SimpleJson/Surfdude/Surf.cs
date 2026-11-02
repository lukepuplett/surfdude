namespace Evoq.Surfdude
{
    using System;
    using System.Linq;
    using Evoq.Surfdude.Hypertext.SimpleJson;

    public sealed class Surf : WaveBuilder
    {
        public Surf(SurfContext journeyContext)
            : this(journeyContext, httpClient: null)
        {

        }

        /// <summary>
        /// Starts a journey that sends its requests through <paramref name="httpClient"/>. A null client
        /// means the shared default.
        /// </summary>
        public Surf(SurfContext journeyContext, System.Net.Http.HttpClient httpClient)
            : base(journeyContext, CreateStepFactory(journeyContext, httpClient))
        {

        }

        private static StepFactory CreateStepFactory(SurfContext journeyContext, System.Net.Http.HttpClient httpClient)
        {
            return new StepFactory(journeyContext, new SimpleJsonResourceReaderWriter(journeyContext), httpClient);
        }

        public static IWaveStart Wave(string rootUri)
        {
            if (string.IsNullOrWhiteSpace(rootUri))
            {
                throw new ArgumentNullOrWhitespaceException(nameof(rootUri));
            }

            return Wave(CreateContext(rootUri));
        }

        public static IWaveStart Wave(SurfContext journeyContext)
        {
            if (journeyContext == null)
            {
                throw new ArgumentNullException(nameof(journeyContext));
            }

            return new Surf(journeyContext);
        }

        private static SurfContext CreateContext(string rootUri)
        {
            return new SurfContext(rootUri);
        }
    }
}
