namespace Evoq.Surfdude.Hypertext.AgentTrader
{
    using System;
    using System.Collections.Generic;
    using System.Linq;
    using System.Text.Json;

    /// <summary>
    /// Reads the AgentTrader envelope:
    /// <c>{"status","message","notes","data":[...],"paging","_links":[...],"_actions":[...]}</c>.
    /// Items in <c>data</c> can carry their own <c>_links</c> and <c>_actions</c>.
    /// Only what the core model can hold is read: the controls. Status, notes, paging and problems
    /// have nowhere to go yet.
    /// </summary>
    internal static class AgentTraderDocumentReader
    {
        // The keys the model has a member for. Any other key on a link or action is an extra.
        private static readonly HashSet<string> LinkCore = new HashSet<string> { "rel", "href", "method" };

        private static readonly HashSet<string> ActionCore = new HashSet<string> { "rel", "action", "method", "enctype", "ifMatch", "fields" };

        public static AgentTraderResource Read(JsonElement root, Uri baseUri)
        {
            var items = new List<AgentTraderControlSet>();

            if (root.TryGetProperty("data", out var data) && data.ValueKind == JsonValueKind.Array)
            {
                foreach (var item in data.EnumerateArray())
                {
                    items.Add(new AgentTraderControlSet(ReadControls(item, baseUri)));
                }
            }

            var missing = root.TryGetProperty("missing", out var missingEl) && missingEl.ValueKind == JsonValueKind.String
                ? missingEl.GetString()
                : string.Empty;

            return new AgentTraderResource(new AgentTraderControlSet(ReadControls(root, baseUri)), items, missing);
        }

        //

        private static List<AgentTraderControl> ReadControls(JsonElement holder, Uri baseUri)
        {
            var controls = new List<AgentTraderControl>();

            if (holder.ValueKind != JsonValueKind.Object)
            {
                return controls; // an item that is not an object simply has no controls
            }

            foreach (var link in Array(holder, "_links"))
            {
                controls.Add(ReadLink(link, baseUri));
            }

            foreach (var action in Array(holder, "_actions"))
            {
                controls.Add(ReadAction(action, baseUri));
            }

            return controls;
        }

        private static AgentTraderControl ReadLink(JsonElement link, Uri baseUri)
        {
            return new AgentTraderControl
            {
                Rel = Text(link, "rel"),
                Method = Text(link, "method"),
                HRef = Resolve(baseUri, Text(link, "href")),
                Inputs = null,
                ControlData = Extras(link, LinkCore),
            };
        }

        private static AgentTraderControl ReadAction(JsonElement action, Uri baseUri)
        {
            return new AgentTraderControl
            {
                Rel = Text(action, "rel"),
                Method = Text(action, "method"),
                Enctype = Text(action, "enctype"),
                IfMatch = Text(action, "ifMatch"),
                HRef = Resolve(baseUri, Text(action, "action")),
                Inputs = Array(action, "fields").Select(ReadField).ToArray(),
                ControlData = Extras(action, ActionCore),
            };
        }

        private static IHypertextInputControl ReadField(JsonElement field)
        {
            string schemaType = null;
            if (field.TryGetProperty("schema", out var schema) && schema.ValueKind == JsonValueKind.Object)
            {
                schemaType = Text(schema, "type");
            }

            return new AgentTraderInput
            {
                Name = Text(field, "name"),
                IsOptional = !(field.TryGetProperty("required", out var required) && required.ValueKind == JsonValueKind.True),
                Value = field.TryGetProperty("value", out var value) ? ValueText(value) : null,
                SchemaType = schemaType,
            };
        }

        private static string ValueText(JsonElement value)
        {
            switch (value.ValueKind)
            {
                case JsonValueKind.Null:
                case JsonValueKind.Undefined:
                    return null;
                case JsonValueKind.String:
                    return value.GetString();
                default:
                    return value.GetRawText(); // 3000, true, or an object or array as JSON text
            }
        }

        private static string Resolve(Uri baseUri, string href)
        {
            if (string.IsNullOrEmpty(href) || baseUri == null)
            {
                return href;
            }

            return new Uri(baseUri, href).AbsoluteUri;
        }

        private static string Text(JsonElement holder, string name)
        {
            return holder.TryGetProperty(name, out var value) && value.ValueKind == JsonValueKind.String
                ? value.GetString()
                : null;
        }

        private static IEnumerable<JsonElement> Array(JsonElement holder, string name)
        {
            return holder.TryGetProperty(name, out var value) && value.ValueKind == JsonValueKind.Array
                ? value.EnumerateArray().ToArray()
                : Enumerable.Empty<JsonElement>();
        }

        // Every key the model has no member for, kept as the JSON text the server sent.
        private static IEnumerable<KeyValuePair<string, string>> Extras(JsonElement holder, HashSet<string> core)
        {
            return holder.EnumerateObject()
                .Where(p => !core.Contains(p.Name) && p.Value.ValueKind != JsonValueKind.Null)
                .Select(p => new KeyValuePair<string, string>(p.Name, p.Value.GetRawText()))
                .ToArray();
        }
    }
}
