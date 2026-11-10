namespace Evoq.Surfdude
{
    using Evoq.Surfdude.Hypertext.Http;
    using Microsoft.Extensions.Logging;
    using System.Collections.Generic;
    using System.Linq;
    using System.Threading;

    public class SurfContext
    {
        public SurfContext(string rootUri, ILoggerFactory loggerFactory = null)
        {
            if (string.IsNullOrWhiteSpace(rootUri))
            {
                throw new ArgumentNullOrWhitespaceException(nameof(rootUri));
            }
            
            this.RootUri = rootUri;
            this.LoggerFactory = loggerFactory;
        }
        
        //

        public string RootUri { get; }

        public int[] ExpectedStatusCodes { get; set; } = Enumerable.Range(start: 200, count: 200).ToArray();

        public ILoggerFactory LoggerFactory { get; }

        public bool ThrowOnError { get; set; } = true;

        /// <summary>
        /// Pluggable challenge handlers tried on unexpected statuses before the step fails.
        /// </summary>
        public IReadOnlyList<Challenge> Challenges { get; set; } = new List<Challenge>();
    }
}