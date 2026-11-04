namespace Evoq.Surfdude.Hypertext.AgentTrader
{
    internal sealed class AgentTraderInput : IHypertextInputControl
    {
        public string Name { get; set; }

        public bool IsOptional { get; set; }

        public string Value { get; set; }

        /// <summary>The JSON Schema <c>type</c> of the field, used to send a number as a number.</summary>
        public string SchemaType { get; set; }
    }
}
