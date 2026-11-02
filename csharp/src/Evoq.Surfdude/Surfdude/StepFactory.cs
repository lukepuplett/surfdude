namespace Evoq.Surfdude
{
    using Evoq.Surfdude.Hypertext;
    using Evoq.Surfdude.Hypertext.Http;
    using Microsoft.Extensions.Logging;
    using System;
    using System.Net.Http;
    using System.Threading;
    using System.Threading.Tasks;

    public class StepFactory // Refactor into IServiceProvider.Get<FromRootStep>()
    {
        // One client for every wave that is not given its own. An HttpClient is meant to be shared: a new
        // one per wave leaves a socket in TIME_WAIT for each, which runs out under load.
        private static readonly Lazy<HttpClient> DefaultHttpClient = new Lazy<HttpClient>(() => new HttpClient());

        public StepFactory(SurfContext context, IHypertextResourceFormatter resourceFormatter)
            : this(context, resourceFormatter, httpClient: null)
        {
        }

        /// <summary>
        /// Creates a factory that sends every request through <paramref name="httpClient"/>, so the caller
        /// controls the handler pipeline: timeouts, retries, authentication, proxies. A null client means
        /// the shared default.
        /// </summary>
        public StepFactory(SurfContext context, IHypertextResourceFormatter resourceFormatter, HttpClient httpClient)
        {
            this.SurfContext = context ?? throw new ArgumentNullException(nameof(context));
            this.ResourceFormatter = resourceFormatter ?? throw new ArgumentNullException(nameof(resourceFormatter));
            this.HttpClient = httpClient ?? DefaultHttpClient.Value;
        }

        //

        public SurfContext SurfContext { get; }

        private IHypertextResourceFormatter ResourceFormatter { get; }

        internal HttpClient HttpClient { get; }

        //

        protected internal virtual IStep GetFromStep(string uri, SurfContext context)
        {
            return new FromStep(this.CreateStepContext(context), uri ?? context.RootUri.ToString());
        }


        protected internal virtual IStep GetToStep(string rel, SurfContext context)
        {
            return new ToStep(this.CreateStepContext(context), rel);
        }

        protected internal virtual IStep GetToItemStep(int index, SurfContext context)
        {
            return new ToItemStep(this.CreateStepContext(context), index);
        }

        protected internal virtual IStep GetSubmitStep(string rel, object transferObject, SurfContext context)
        {
            return new SubmitStep(this.CreateStepContext(context), rel, transferObject);
        }

        internal ReadStep<TModel> GetReadStep<TModel>(SurfContext context, TModel[] models) where TModel : class
        {
            return new ReadStep<TModel>(this.CreateStepContext(context), models);
        }

        private HttpStepContext CreateStepContext(SurfContext context)
        {
            return new HttpStepContext(this.HttpClient, context, this.ResourceFormatter);
        }
    }
}