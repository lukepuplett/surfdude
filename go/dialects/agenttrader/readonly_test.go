package agenttrader_test

import (
	"net/http"
	"testing"

	"github.com/lukepuplett/surfdude/go/dialects/agenttrader"
)

// Scenario: a field whose schema says readOnly: true is read as a read-only input, and others are not
//
//	Go only for now: the shared fixtures are also read by the C# client, which does not read readOnly yet.
func TestAgentTrader_ReadsReadOnlyFromTheFieldSchema(t *testing.T) {
	body := []byte(`{"status":200,"message":"OK","data":[],"_actions":[{"rel":"verify-email","action":"/auth/email/verify","method":"POST","fields":[
		{"name":"email","required":true,"value":"sally@seller.example","schema":{"type":"string","readOnly":true}},
		{"name":"code","required":true,"schema":{"type":"string"}},
		{"name":"note","schema":{"readOnly":false}}]}]}`)
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}}

	res, err := agenttrader.New().Read(resp, body)

	if err != nil {
		t.Fatal(err)
	}
	c, ok := res.Control("verify-email")
	if !ok || len(c.Inputs) != 3 {
		t.Fatalf("control = %+v, want verify-email with three inputs", c)
	}
	for _, want := range []struct {
		name     string
		readOnly bool
	}{{"email", true}, {"code", false}, {"note", false}} {
		for _, in := range c.Inputs {
			if in.Name == want.name && in.ReadOnly() != want.readOnly {
				t.Errorf("%s.ReadOnly() = %v, want %v", want.name, in.ReadOnly(), want.readOnly)
			}
		}
	}
	if c.Inputs[0].Value != "sally@seller.example" {
		t.Errorf("the read-only input lost its value: %v", c.Inputs[0].Value)
	}
}
