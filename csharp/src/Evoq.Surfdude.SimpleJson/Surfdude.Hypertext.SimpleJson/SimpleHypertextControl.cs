namespace Evoq.Surfdude.Hypertext.SimpleJson
{
    using Evoq.Surfdude.Hypertext.Http;
    using System.Collections.Generic;
    using System.Linq;
    
    internal class SimpleHypertextControl : IHypertextControl
    {
        public string HRef { get; set; }

        public string Rel { get; set; }

        public string Method { get; set; }

        public string IfMatch { get; set; }

        public IEnumerable<SimpleInputControl> Form { get; set; }

        public IEnumerable<IHypertextInputControl> Inputs => this.Form;

        // The simple dialect has no extra parts: its method and if-match are the members above.
        public IEnumerable<KeyValuePair<string, string>> ControlData => System.Array.Empty<KeyValuePair<string, string>>();

        public bool RequiresInput => this.Inputs?.Any(i => !i.IsOptional) ?? false;
    }
}