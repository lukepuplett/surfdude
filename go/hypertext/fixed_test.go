package hypertext

import (
	"reflect"
	"testing"
)

// Fixed and editable values (docs/model.md): a field says "change this if you like", and a value
// with no field says "this is decided". Two facts about a control follow from that, and are read here.

// Scenario: an input is read-only when its schema says readOnly: true, and only then
func TestInput_ReadOnly(t *testing.T) {
	cases := map[string]struct {
		schema Schema
		want   bool
	}{
		"readOnly true":         {Schema{"readOnly": true}, true},
		"readOnly false":        {Schema{"readOnly": false}, false},
		"readOnly as text":      {Schema{"readOnly": "true"}, false},
		"no readOnly":           {Schema{"type": "string"}, false},
		"no schema":             {nil, false},
		"true among other keys": {Schema{"type": "string", "readOnly": true, "format": "email"}, true},
	}

	for name, c := range cases {
		if got := (Input{Name: "email", Schema: c.schema}).ReadOnly(); got != c.want {
			t.Errorf("%s: ReadOnly() = %v, want %v", name, got, c.want)
		}
	}
}

// Scenario: an input repeats a fixed parameter when the same name is a literal query parameter of the href
func TestControl_RepeatedFixedParams(t *testing.T) {
	inputs := func(names ...string) []Input {
		var out []Input
		for _, n := range names {
			out = append(out, Input{Name: n})
		}
		return out
	}
	cases := map[string]struct {
		href   string
		inputs []Input
		want   []string
	}{
		"a fixed to and an input to":        {"/sandbox/outbox?to=a%40b.test", inputs("to"), []string{"to"}},
		"a fixed value that is empty":       {"/x?to=", inputs("to"), []string{"to"}},
		"two clashes, in input order":       {"/x?b=1&a=2", inputs("a", "b", "c"), []string{"a", "b"}},
		"a different name does not clash":   {"/x?to=a", inputs("from"), nil},
		"no query at all":                   {"/x", inputs("to"), nil},
		"a template is not a fixed value":   {"/x{?to}", inputs("to"), nil},
		"a fixed value beside a template":   {"/x?to=1{&from}", inputs("to", "from"), []string{"to"}},
		"a placeholder value is not fixed":  {"/x?q={q}", inputs("q"), nil},
		"a full URL works the same":         {"https://example.test/x?to=a", inputs("to"), []string{"to"}},
		"an href that cannot be read":       {"://bad url?to=a", inputs("to"), nil},
		"no inputs":                         {"/x?to=a", nil, nil},
		"names are exact, not by substring": {"/x?tomorrow=1", inputs("to"), nil},
	}

	for name, c := range cases {
		got := (Control{Rel: "r", Href: c.href, Inputs: c.inputs}).RepeatedFixedParams()
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: RepeatedFixedParams() = %v, want %v", name, got, c.want)
		}
	}
}
