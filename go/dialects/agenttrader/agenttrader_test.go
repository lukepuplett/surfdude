package agenttrader_test

import (
	"net/http"
	"testing"

	"github.com/lukepuplett/surfdude/go/dialects/agenttrader"
	"github.com/lukepuplett/surfdude/go/internal/conformance"
)

// The envelope dialect reads the samples our API is designed around, through the same neutral model.
func TestAgentTrader_Conformance(t *testing.T) {
	conformance.Run(t, agenttrader.New(), "../../../fixtures/agenttrader")
}

// Scenario: a body that is not the envelope is not claimed
//
//	Given JSON without a status and a data list, and no profile link
//	Then the dialect does not match it, so another dialect can have it
func TestAgentTrader_DoesNotClaimOtherJSON(t *testing.T) {
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}}

	if agenttrader.New().Matches(resp, []byte(`{"links":[]}`)) {
		t.Error("JSON that is not the envelope must not be claimed")
	}
	if agenttrader.New().Matches(&http.Response{Header: http.Header{"Content-Type": {"text/html"}}}, []byte(`<html></html>`)) {
		t.Error("HTML must not be claimed")
	}
}

// Scenario: what the model has no field for is kept as an extension, as the server sent it
//
//	Given an action with a price, a title and a requirement, and a link with a type
//	Then each is an extension holding the server's JSON, and the parts that change the request are not
func TestAgentTrader_KeepsOtherKeysAsExtensions(t *testing.T) {
	body := []byte(`{"status":200,"message":"OK","data":[],
		"_links":[{"rel":"human","href":"/h","method":"GET","type":"text/html"}],
		"_actions":[{"rel":"buy","action":"/buy","method":"POST","enctype":"application/json","ifMatch":"v1",
			"title":"Buy it","requires":["agent-session"],"price":{"amount":"0.50","currency":"USDC"},"fields":[]}]}`)
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}}

	res, err := agenttrader.New().Read(resp, body)
	if err != nil {
		t.Fatal(err)
	}

	buy, _ := res.Control("buy")
	for _, want := range []string{"title", "requires", "price"} {
		if _, ok := buy.Extension(want); !ok {
			t.Errorf("the buy action should have the extension %q, it has %v", want, buy.Extensions)
		}
	}
	for _, core := range []string{"rel", "action", "method", "enctype", "ifMatch", "fields"} {
		if _, ok := buy.Extension(core); ok {
			t.Errorf("%q changes the request, so it must be a field and not an extension", core)
		}
	}
	var price struct{ Amount string }
	if found, err := buy.DecodeExtension("price", &price); !found || err != nil || price.Amount != "0.50" {
		t.Errorf("price = %+v (found %v, err %v), want amount 0.50", price, found, err)
	}
	human, _ := res.Control("human")
	if raw, _ := human.Extension("type"); string(raw) != `"text/html"` {
		t.Errorf("type = %s, want \"text/html\"", raw)
	}
}
