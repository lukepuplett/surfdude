namespace Evoq.Surfdude.Hypertext.AgentTrader
{
    /// <summary>
    /// The keys this dialect puts in a control's <see cref="IHypertextControl.ControlData"/>. They are the
    /// control's extra parts: the core model does not know them, and nothing in the core acts on them.
    /// Each value is the JSON text the server sent, so a list stays a list and an object stays an object.
    /// </summary>
    public static class AgentTraderControlData
    {
        /// <summary>What the caller needs first, a JSON array such as <c>["agent-session"]</c>.</summary>
        public static string Requires { get; } = "requires";

        /// <summary>A payment the control mentions, a JSON object such as <c>{"amount":"0.50","currency":"USDC"}</c>.</summary>
        public static string Price { get; } = "price";

        public static string Title { get; } = "title";

        /// <summary>The media type the control serves when it is not hypermedia.</summary>
        public static string Type { get; } = "type";
    }
}
