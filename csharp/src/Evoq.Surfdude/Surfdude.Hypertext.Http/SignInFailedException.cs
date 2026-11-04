namespace Evoq.Surfdude.Hypertext.Http
{
    using System;

    /// <summary>
    /// Signing in again failed: the sign-in delegate threw, or it gave no token.
    /// </summary>
    public class SignInFailedException : FailedSurfException
    {
        public SignInFailedException(string message) : base(message)
        {
        }

        public SignInFailedException(string message, Exception innerException) : base(message, innerException)
        {
        }
    }
}
