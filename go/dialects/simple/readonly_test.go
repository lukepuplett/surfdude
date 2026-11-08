package simple_test

import (
	"net/http"
	"testing"

	"github.com/lukepuplett/surfdude/go/dialects/simple"
)

// Scenario: a form field marked readOnly is read as a read-only input, and others are not
//
//	Go only for now: the shared fixtures are also read by the C# client, which does not read readOnly yet.
func TestSimple_ReadsReadOnlyOnAFormField(t *testing.T) {
	body := []byte(`{"links":[{"rel":"verify-email","href":"/verify","method":"POST","form":[
		{"name":"email","value":"sally@seller.example","readOnly":true},
		{"name":"code"},
		{"name":"note","isOptional":true,"readOnly":false}]}]}`)
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {simple.MediaType}}}

	res, err := simple.New().Read(resp, body)

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
