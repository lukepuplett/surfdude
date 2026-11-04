namespace Evoq.Surfdude.Hypertext.AgentTrader
{
    using System;
    using System.Collections.Generic;

    internal sealed class AgentTraderResource : IHypertextResource
    {
        private readonly AgentTraderControlSet controls;
        private readonly IReadOnlyList<AgentTraderControlSet> items;

        public AgentTraderResource(AgentTraderControlSet controls, IReadOnlyList<AgentTraderControlSet> items)
        {
            this.controls = controls ?? throw new ArgumentNullException(nameof(controls));
            this.items = items ?? throw new ArgumentNullException(nameof(items));
        }

        //

        public IEnumerable<string> Rels => this.controls.Rels;

        public IReadOnlyList<AgentTraderControlSet> Items => this.items;

        public IHypertextControl GetControl(string rel) => this.controls.GetControl(rel);

        public IHypertextControls GetItem(int index)
        {
            if (index < 0 || index >= this.items.Count)
            {
                throw new ArgumentOutOfRangeException(
                    nameof(index), $"Unable to select item {index}. The resource has {this.items.Count}.");
            }

            return this.items[index];
        }
    }
}
