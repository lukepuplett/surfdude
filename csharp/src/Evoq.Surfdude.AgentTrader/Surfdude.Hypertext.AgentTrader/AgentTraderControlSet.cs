namespace Evoq.Surfdude.Hypertext.AgentTrader
{
    using System;
    using System.Collections.Generic;
    using System.Linq;

    internal sealed class AgentTraderControlSet : IHypertextControls
    {
        private readonly IReadOnlyList<AgentTraderControl> controls;

        public AgentTraderControlSet(IReadOnlyList<AgentTraderControl> controls)
        {
            this.controls = controls ?? throw new ArgumentNullException(nameof(controls));
        }

        //

        public IEnumerable<string> Rels => this.controls.Select(c => c.Rel);

        public IHypertextControl GetControl(string rel)
        {
            if (rel == null)
            {
                throw new ArgumentNullException(nameof(rel));
            }

            return this.controls.FirstOrDefault(c => rel.Equals(c.Rel, StringComparison.OrdinalIgnoreCase))
                ?? throw new RelationNotFoundException(
                    $"Could not find a hyperlink with relation '{rel}'. The available relations are '{string.Join(", ", this.Rels)}'");
        }
    }
}
