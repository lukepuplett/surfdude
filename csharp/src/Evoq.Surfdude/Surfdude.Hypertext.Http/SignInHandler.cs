namespace Evoq.Surfdude.Hypertext.Http
{
    using System;
    using System.Net;
    using System.Net.Http;
    using System.Net.Http.Headers;
    using System.Threading;
    using System.Threading.Tasks;

    /// <summary>
    /// A handler for the <see cref="HttpClient"/> pipeline that gives a surfer a session. It sends the
    /// access token it holds as <c>Authorization: Bearer</c>. When a request is refused with a 401 it calls
    /// the sign-in delegate, keeps the token that comes back, and sends the same request again, once, with
    /// the body intact.
    /// </summary>
    /// <remarks>
    /// Guards:
    /// <list type="bullet">
    /// <item>A wave that expects 401, by listing it in <see cref="SurfContext.ExpectedStatusCodes"/>, is testing
    /// the refusal and is left alone.</item>
    /// <item>The requests the sign-in delegate sends cannot call it again, so a refused sign-in fails the
    /// step instead of recursing.</item>
    /// <item>One sign-in and one replay per request. If the new token is refused too, the second 401 is
    /// returned as it is.</item>
    /// <item>Requests that fail together share one sign-in: callers take turns, and one that finds the token
    /// already changed just sends again with it.</item>
    /// </list>
    /// A request body is read into memory before it is first sent, so it can be sent again. Do not use this
    /// handler to stream very large uploads.
    /// There is nothing here for a control that needs a session before any request is sent, because the simple
    /// dialect has no such control.
    /// </remarks>
    public sealed class SignInHandler : DelegatingHandler
    {
        /// <summary>
        /// Set on a request that expects a 401, so the handler leaves it alone. The steps of a wave set it
        /// when the wave's expected status codes include 401.
        /// </summary>
        public static readonly HttpRequestOptionsKey<bool> ExpectsUnauthorized = new HttpRequestOptionsKey<bool>("Evoq.Surfdude.ExpectsUnauthorized");

        // Marks the logical flow of the sign-in delegate, so the requests it sends cannot call it again.
        private static readonly AsyncLocal<bool> SigningIn = new AsyncLocal<bool>();

        private readonly SignInDelegate signIn;
        private readonly SemaphoreSlim turn = new SemaphoreSlim(1, 1);
        private string token;

        public SignInHandler(SignInDelegate signIn, string token = null)
        {
            this.signIn = signIn ?? throw new ArgumentNullException(nameof(signIn));
            this.token = token;
        }

        //

        /// <summary>
        /// The access token now held, or null if none.
        /// </summary>
        public string Token => Volatile.Read(ref this.token);

        //

        protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            var replayable = await ReplayableRequest.CaptureAsync(request, cancellationToken);

            string used = this.Token;
            if (request.Headers.Authorization == null && !string.IsNullOrEmpty(used))
            {
                request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", used);
            }

            var response = await base.SendAsync(request, cancellationToken);

            if (response.StatusCode != HttpStatusCode.Unauthorized
                || SigningIn.Value
                || (request.Options.TryGetValue(ExpectsUnauthorized, out bool expected) && expected))
            {
                return response;
            }

            await response.Content.LoadIntoBufferAsync();
            await this.SignInAgainAsync(response, request.Headers.Authorization?.Parameter, cancellationToken);
            response.Dispose();

            var again = replayable.CreateAgain();
            again.Headers.Authorization = new AuthenticationHeaderValue("Bearer", this.Token);

            return await base.SendAsync(again, cancellationToken);
        }

        protected override void Dispose(bool disposing)
        {
            if (disposing)
            {
                this.turn.Dispose();
            }

            base.Dispose(disposing);
        }

        //

        // Runs the delegate, one caller at a time. stale is the token the failed request used. If the token
        // has changed by the time this caller gets its turn, another request has already signed in, and
        // there is nothing to do but send again.
        private async Task SignInAgainAsync(HttpResponseMessage unauthorized, string stale, CancellationToken cancellationToken)
        {
            await this.turn.WaitAsync(cancellationToken);
            try
            {
                if ((this.Token ?? string.Empty) != (stale ?? string.Empty))
                {
                    return;
                }

                string newToken;

                SigningIn.Value = true;
                try
                {
                    newToken = await this.signIn(unauthorized, cancellationToken);
                }
                catch (Exception exception) when (exception is not OperationCanceledException)
                {
                    throw new SignInFailedException(
                        $"Unable to send the request again. Signing in again failed: {exception.Message}", exception);
                }
                finally
                {
                    SigningIn.Value = false;
                }

                if (string.IsNullOrEmpty(newToken))
                {
                    throw new SignInFailedException(
                        "Unable to send the request again. Signing in again gave no token.");
                }

                Volatile.Write(ref this.token, newToken);
            }
            finally
            {
                this.turn.Release();
            }
        }
    }
}
