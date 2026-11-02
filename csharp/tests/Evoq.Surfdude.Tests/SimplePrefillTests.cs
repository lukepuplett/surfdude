namespace Evoq.Surfdude.Tests
{
    using Evoq.Surfdude.Hypertext.SimpleJson;
    using System.Collections.Generic;
    using System.Linq;
    using System.Net;
    using System.Net.Http;
    using System.Text;
    using System.Threading.Tasks;
    using Xunit;

    public class SimplePrefillTests
    {
        [Fact]
        public async Task ReadAsResourceAsync_maps_value_on_form_inputs()
        {
            const string json = """
                {
                  "links": [{
                    "rel": "save-draft",
                    "href": "https://example.test/drafts",
                    "method": "POST",
                    "form": [
                      {"name": "sessionToken", "value": "tok_abc"},
                      {"name": "title"}
                    ]
                  }]
                }
                """;
            var formatter = new SimpleJsonResourceReaderWriter(new SurfContext("https://example.test/"));
            var response = new HttpResponseMessage(HttpStatusCode.OK)
            {
                Content = new StringContent(json, Encoding.UTF8, "application/simple-hypertext+json")
            };

            var resource = await formatter.ReadAsResourceAsync(response);
            var control = resource.GetControl("save-draft");
            var token = control.Inputs.First(i => i.Name == "sessionToken");

            Assert.Equal("tok_abc", token.Value);
        }

        [Fact]
        public async Task BuildRequest_includes_prefilled_values_when_submit_omits_them()
        {
            const string json = """
                {
                  "links": [{
                    "rel": "save-draft",
                    "href": "https://example.test/drafts",
                    "method": "POST",
                    "form": [
                      {"name": "sessionToken", "value": "tok_abc"},
                      {"name": "title"}
                    ]
                  }]
                }
                """;
            var formatter = new SimpleJsonResourceReaderWriter(new SurfContext("https://example.test/"));
            var response = new HttpResponseMessage(HttpStatusCode.OK)
            {
                Content = new StringContent(json, Encoding.UTF8, "application/simple-hypertext+json")
            };
            var resource = await formatter.ReadAsResourceAsync(response);
            var control = resource.GetControl("save-draft");

            var request = formatter.BuildRequest(
                new Dictionary<string, string> { ["title"] = "Hello" },
                control);

            var body = await request.Content!.ReadAsStringAsync();
            Assert.Contains("tok_abc", body);
            Assert.Contains("Hello", body);
        }

        [Fact]
        public void BuildRequest_caller_value_overrides_prefill()
        {
            var control = new SimpleHypertextControl
            {
                Rel = "save",
                HRef = "https://example.test/save",
                Method = "POST",
                Form = new[]
                {
                    new SimpleInputControl { Name = "sessionToken", Value = "tok_abc" },
                    new SimpleInputControl { Name = "title" },
                }
            };
            var formatter = new SimpleJsonResourceReaderWriter(new SurfContext("https://example.test/"));

            var request = formatter.BuildRequest(
                new Dictionary<string, string> { ["sessionToken"] = "override", ["title"] = "Hi" },
                control);

            var body = request.Content!.ReadAsStringAsync().GetAwaiter().GetResult();
            Assert.Contains("override", body);
            Assert.DoesNotContain("tok_abc", body);
        }
    }
}
