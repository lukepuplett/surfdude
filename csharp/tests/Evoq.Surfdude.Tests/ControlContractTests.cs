namespace Evoq.Surfdude.Tests
{
    using Evoq.Surfdude.Hypertext;
    using Evoq.Surfdude.Hypertext.AgentTrader;
    using Evoq.Surfdude.Hypertext.Http;

    /// <summary>
    /// What changes the request has its own member on the control. The bag holds extras the core never reads.
    /// </summary>
    public class ControlContractTests
    {
        private sealed class LegacyControl : IHypertextControl
        {
            public string HRef => "https://example.test/x";

            public string Rel => "x";

            public IEnumerable<IHypertextInputControl> Inputs => null;

            public IEnumerable<KeyValuePair<string, string>> ControlData { get; } = new[]
            {
                new KeyValuePair<string, string>(HttpControlData.MethodControlName, "put"),
                new KeyValuePair<string, string>(HttpControlData.IfMatchControlName, "v9"),
            };
        }

        [Fact]
        public void IHypertextControl__Method__when_a_control_keeps_it_in_the_bag_the_old_way__then_the_default_still_finds_it()
        {
            IHypertextControl control = new LegacyControl();

            Assert.Equal("put", control.Method);
            Assert.Equal("v9", control.IfMatch);
            Assert.Equal("PUT", control.DetermineHttpMethod().Method);
        }

        [Fact]
        public async Task AgentTrader__ControlData__when_a_control_has_a_method_and_a_version__then_the_bag_holds_neither()
        {
            var resource = (AgentTraderResource)await new AgentTraderResourceFormatter().ReadAsResourceAsync(Fixtures.Response("listing-show.json"));

            var control = resource.GetControl("change-price");

            Assert.Equal("PATCH", control.Method);
            Assert.Equal("v3", control.IfMatch);
            Assert.Equal("application/json", control.Enctype);
            Assert.DoesNotContain(control.ControlData, d => d.Key == HttpControlData.MethodControlName || d.Key == HttpControlData.IfMatchControlName);
        }
    }
}
