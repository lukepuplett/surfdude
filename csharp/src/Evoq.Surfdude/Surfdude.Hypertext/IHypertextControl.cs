namespace Evoq.Surfdude.Hypertext
{
    using Evoq.Surfdude.Hypertext.Http;
    using System.Collections.Generic;
    using System.Linq;

    public interface IHypertextControl : IHyperlink
    {
        IEnumerable<IHypertextInputControl> Inputs { get; }

        /// <summary>
        /// The control's extra parts: whatever a dialect found that the core model has no member for, keyed by
        /// the dialect's own name, each value the text the server sent. The core never reads it. What
        /// changes the request has its own member: <see cref="Method"/>, <see cref="Enctype"/>, <see cref="IfMatch"/>.
        /// </summary>
        IEnumerable<KeyValuePair<string, string>> ControlData { get; }

        /// <summary>The HTTP method, or null for GET.</summary>
        /// <remarks>
        /// Controls written before this member existed kept the method in <see cref="ControlData"/> under
        /// the key "method", and this default still finds it there.
        /// </remarks>
        string Method => this.ControlData?.FirstOrDefault(d => d.Key == HttpControlData.MethodControlName).Value;

        /// <summary>The media type of the body to send, or null for the dialect's default.</summary>
        string Enctype => null;

        /// <summary>The version the control expects, sent as If-Match, or null.</summary>
        string IfMatch => this.ControlData?.FirstOrDefault(d => d.Key == HttpControlData.IfMatchControlName).Value;
    }
}
