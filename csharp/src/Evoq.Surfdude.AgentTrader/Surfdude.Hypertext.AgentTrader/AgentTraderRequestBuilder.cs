namespace Evoq.Surfdude.Hypertext.AgentTrader
{
    using Evoq.Surfdude.Hypertext.Http;
    using System;
    using System.Collections.Generic;
    using System.Linq;
    using System.Net.Http;
    using System.Text;
    using System.Text.Json;

    /// <summary>
    /// Builds the one HTTP request for a control: the server's prefilled values first, then the journey's
    /// values over them, checked against the control's inputs before anything is sent.
    /// </summary>
    internal static class AgentTraderRequestBuilder
    {
        private const string JsonMediaType = "application/json";

        public static HttpRequestMessage Build(IDictionary<string, string> sendPairs, IHypertextControl control)
        {
            if (sendPairs == null)
            {
                throw new ArgumentNullException(nameof(sendPairs));
            }

            if (control == null)
            {
                throw new ArgumentNullException(nameof(control));
            }

            var inputs = control.Inputs?.ToArray();
            var merged = Merge(sendPairs, inputs);

            Check(control, inputs, merged);

            var method = control.DetermineHttpMethod();
            var request = control.SupportsRequestBody() || method == HttpMethod.Patch
                ? new HttpRequestMessage(method, control.HRef) { Content = Body(control, inputs, merged) }
                : new HttpRequestMessage(method, WithQuery(control.HRef, merged));

            request.Headers.Accept.ParseAdd(JsonMediaType);

            if (control.TryParseIfMatch(out string ifMatch))
            {
                // The version is whatever the server called it, not necessarily a quoted entity tag.
                request.Headers.TryAddWithoutValidation("If-Match", ifMatch);
            }

            return request;
        }

        //

        private static Dictionary<string, string> Merge(IDictionary<string, string> sendPairs, IHypertextInputControl[] inputs)
        {
            var merged = new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase);

            foreach (var input in inputs ?? Array.Empty<IHypertextInputControl>())
            {
                if (input.Value != null)
                {
                    merged[input.Name] = input.Value;
                }
            }

            foreach (var pair in sendPairs)
            {
                merged[pair.Key] = pair.Value;
            }

            return merged;
        }

        private static void Check(IHypertextControl control, IHypertextInputControl[] inputs, Dictionary<string, string> merged)
        {
            var known = new HashSet<string>((inputs ?? Array.Empty<IHypertextInputControl>()).Select(i => i.Name), StringComparer.OrdinalIgnoreCase);
            var unexpected = merged.Keys.Where(k => !known.Contains(k)).ToArray();

            if (unexpected.Length > 0)
            {
                throw new UnexpectedInputsException(
                    $"Unable to prepare a request for the relation '{control.Rel}'. These inputs are not on the control, " +
                    $"'{string.Join(", ", unexpected)}'. The control has '{string.Join(", ", known)}'.");
            }

            var missing = (inputs ?? Array.Empty<IHypertextInputControl>())
                .Where(i => !i.IsOptional && !merged.ContainsKey(i.Name))
                .Select(i => i.Name)
                .ToArray();

            if (missing.Length > 0)
            {
                throw new MissingInputException(
                    $"Unable to prepare a request for the relation '{control.Rel}'. These required inputs were " +
                    $"not supplied, '{string.Join(", ", missing)}'.");
            }
        }

        private static string WithQuery(string href, Dictionary<string, string> merged)
        {
            if (merged.Count == 0)
            {
                return href;
            }

            string query = string.Join("&", merged.Select(p => $"{Uri.EscapeDataString(p.Key)}={Uri.EscapeDataString(p.Value)}"));

            return href + (href.Contains('?') ? "&" : "?") + query;
        }

        private static HttpContent Body(IHypertextControl control, IHypertextInputControl[] inputs, Dictionary<string, string> merged)
        {
            string enctype = control.Enctype;

            if (enctype != null && enctype.StartsWith("application/x-www-form-urlencoded", StringComparison.OrdinalIgnoreCase))
            {
                return new FormUrlEncodedContent(merged);
            }

            var tree = new Dictionary<string, object>();

            foreach (var pair in merged)
            {
                string schemaType = inputs?
                    .OfType<AgentTraderInput>()
                    .FirstOrDefault(i => string.Equals(i.Name, pair.Key, StringComparison.OrdinalIgnoreCase))?.SchemaType;

                Nest(tree, pair.Key.Split('.'), Typed(pair.Value, schemaType));
            }

            return new StringContent(JsonSerializer.Serialize(tree), Encoding.UTF8, enctype ?? JsonMediaType);
        }

        // A dotted name such as location.lat becomes a nested object in the body.
        private static void Nest(Dictionary<string, object> tree, string[] path, object value)
        {
            for (int i = 0; i < path.Length - 1; i++)
            {
                if (!(tree.TryGetValue(path[i], out var next) && next is Dictionary<string, object> child))
                {
                    child = new Dictionary<string, object>();
                    tree[path[i]] = child;
                }

                tree = child;
            }

            tree[path[path.Length - 1]] = value;
        }

        // Values travel as text. The schema says which ones are numbers or booleans in the JSON body.
        private static object Typed(string text, string schemaType)
        {
            switch (schemaType)
            {
                case "integer" when long.TryParse(text, out long whole):
                    return whole;
                case "number" when double.TryParse(text, System.Globalization.NumberStyles.Float, System.Globalization.CultureInfo.InvariantCulture, out double number):
                    return number;
                case "boolean" when bool.TryParse(text, out bool flag):
                    return flag;
                default:
                    return text;
            }
        }
    }
}
