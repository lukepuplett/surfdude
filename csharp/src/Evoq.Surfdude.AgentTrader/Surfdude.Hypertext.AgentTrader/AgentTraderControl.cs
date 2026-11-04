namespace Evoq.Surfdude.Hypertext.AgentTrader
{
    using System.Collections.Generic;

    internal sealed class AgentTraderControl : IHypertextControl
    {
        public string HRef { get; set; }

        public string Rel { get; set; }

        public string Method { get; set; }

        public string Enctype { get; set; }

        public string IfMatch { get; set; }

        /// <summary>The form's fields, or null for a plain link.</summary>
        public IEnumerable<IHypertextInputControl> Inputs { get; set; }

        public IEnumerable<KeyValuePair<string, string>> ControlData { get; set; }
    }
}
