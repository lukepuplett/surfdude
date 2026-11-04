namespace Evoq.Surfdude.Hypertext.Http
{
    using System.Net.Http;
    using System.Threading;
    using System.Threading.Tasks;

    /// <summary>
    /// Signs in and returns the access token to use.
    /// </summary>
    /// <param name="unauthorized">
    /// The 401 response that showed the need. Its body is buffered, so it can be read. It carries the
    /// server's advice and, if the API offers one, a sign-in control to follow.
    /// </param>
    /// <param name="cancellationToken">Cancels the sign-in.</param>
    public delegate Task<string> SignInDelegate(HttpResponseMessage unauthorized, CancellationToken cancellationToken);
}
