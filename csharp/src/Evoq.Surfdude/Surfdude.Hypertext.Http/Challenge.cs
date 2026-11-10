namespace Evoq.Surfdude.Hypertext.Http
{
    using Evoq.Surfdude.Hypertext;
    using System;
    using System.Net;
    using System.Net.Http;
    using System.Threading;
    using System.Threading.Tasks;

    /// <summary>
    /// Says whether a response is a challenge Surfdude should handle instead of failing the step.
    /// </summary>
    public delegate bool ChallengeMatcher(HttpStatusCode status, HttpResponseHeaders headers, byte[] body);

    /// <summary>
    /// Runs the side work for a challenge. When it returns true, the step replays the original request once.
    /// </summary>
    public delegate Task<bool> ChallengeHandler(ChallengeContext context, CancellationToken cancellationToken);

    /// <summary>
    /// One registered response pattern and how to recover from it.
    /// </summary>
    public sealed class Challenge
    {
        public Challenge(string name, ChallengeMatcher match, ChallengeHandler handle)
        {
            this.Name = name ?? throw new ArgumentNullException(nameof(name));
            this.Match = match ?? throw new ArgumentNullException(nameof(match));
            this.Handle = handle ?? throw new ArgumentNullException(nameof(handle));
        }

        public string Name { get; }

        public ChallengeMatcher Match { get; }

        public ChallengeHandler Handle { get; }
    }

    /// <summary>
    /// What a handler receives: the refused page, and enough context to follow a recovery control.
    /// </summary>
    public sealed class ChallengeContext
    {
        public ChallengeContext(
            HttpStepContext stepContext,
            IHypertextResource page,
            HttpResponseMessage refused,
            byte[] body)
        {
            this.StepContext = stepContext ?? throw new ArgumentNullException(nameof(stepContext));
            this.Page = page ?? throw new ArgumentNullException(nameof(page));
            this.Refused = refused ?? throw new ArgumentNullException(nameof(refused));
            this.Body = body ?? Array.Empty<byte>();
        }

        public HttpStepContext StepContext { get; }

        public IHypertextResource Page { get; }

        public HttpResponseMessage Refused { get; }

        public byte[] Body { get; }
    }

    internal static class ChallengeRunner
    {
        private static readonly SemaphoreSlim Gate = new SemaphoreSlim(1, 1);

        internal static async Task<bool> TryAsync(
            HttpStepContext stepContext,
            IHypertextResource refusedResource,
            HttpResponseMessage refused,
            byte[] body,
            CancellationToken cancellationToken)
        {
            var challenges = stepContext.SurfContext.Challenges;
            if (challenges == null || challenges.Count == 0)
            {
                return false;
            }

            foreach (var challenge in challenges)
            {
                if (!challenge.Match(refused.StatusCode, refused.Headers, body))
                {
                    continue;
                }

                await Gate.WaitAsync(cancellationToken).ConfigureAwait(false);
                try
                {
                    return await challenge.Handle(
                        new ChallengeContext(stepContext, refusedResource, refused, body),
                        cancellationToken).ConfigureAwait(false);
                }
                finally
                {
                    Gate.Release();
                }
            }

            return false;
        }
    }
}
