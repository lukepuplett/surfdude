namespace Evoq.Surfdude
{
    using Evoq.Surfdude.Hypertext.AgentTrader;
    using System.Net.Http;

    /// <summary>
    /// A journey over an API that speaks the AgentTrader envelope.
    /// </summary>
    public sealed class AgentTraderSurf : WaveBuilder
    {
        public AgentTraderSurf(SurfContext journeyContext, HttpClient httpClient)
            : base(journeyContext, new StepFactory(journeyContext, new AgentTraderResourceFormatter(), httpClient))
        {
        }
    }
}
