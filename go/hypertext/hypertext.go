// Package hypertext is the model every dialect reads a response into. It is deliberately small and
// does not care how a server spells its links and forms: a dialect (see package dialect) turns a
// response into these types, and the surf package drives a journey using only them.
//
// The model: a Resource has Controls found by rel. A Control is a link (go somewhere) or a form
// (send something). A form has Inputs, each with an optional JSON Schema fragment that says what is
// allowed. A Resource can also hold Items, and each Item has controls of its own, so "the second
// search result" can offer "shortlist" without the client knowing any URL.
package hypertext

import (
	"encoding/json"
	"strings"
)

// Schema is a JSON Schema fragment describing one input: its type, enum, bounds and so on.
type Schema map[string]any

// Input is one field of a form. Value is the suggested or current value, like an HTML value
// attribute: if the caller does not override it, it is sent.
type Input struct {
	Name     string
	Required bool
	Value    any
	Schema   Schema
}

// Price is shown on a control that needs a payment, for example an x402 amount.
type Price struct {
	Amount   string
	Currency string
	Note     string
}

// Control is something a client can follow or submit. It is found by Rel.
type Control struct {
	Rel   string
	Title string
	Href  string
	// Method is the HTTP method. Empty means GET.
	Method string
	// Enctype is the media type to send the body in. Empty means the dialect's default.
	Enctype string
	// Type is the media type the control serves, when it is not the usual one.
	Type string
	// Form is true when the control carries inputs to send, false for a plain link.
	Form   bool
	Inputs []Input
	// Requires lists what the caller needs first, for example "agent-session".
	Requires []string
	// IfMatch is the version the control expects, sent as If-Match.
	IfMatch string
	Price   *Price
}

// HTTPMethod is the upper-case method, GET when none was given.
func (c Control) HTTPMethod() string {
	if c.Method == "" {
		return "GET"
	}
	return strings.ToUpper(c.Method)
}

// RequiredInputs are the inputs the caller must supply, those that are required and have no value.
func (c Control) RequiredInputs() []Input {
	var out []Input
	for _, in := range c.Inputs {
		if in.Required && in.Value == nil {
			out = append(out, in)
		}
	}
	return out
}

// Input returns the named input.
func (c Control) Input(name string) (Input, bool) {
	for _, in := range c.Inputs {
		if in.Name == name {
			return in, true
		}
	}
	return Input{}, false
}

// Paging describes one page of a list.
type Paging struct {
	Skip, Take, Count int
	// Total is nil when the server did not say.
	Total *int
}

// Problem is one thing wrong with a request, as the server reports it.
type Problem struct {
	Field   string
	Code    string
	Message string
}

// Item is one member of a list. It has controls of its own.
type Item struct {
	Controls []Control
	Notes    []string
	// Data is the item's own JSON.
	Data json.RawMessage
}

// Control finds an item's control by rel.
func (i Item) Control(rel string) (Control, bool) { return find(i.Controls, rel) }

// Rels lists the rels the item offers.
func (i Item) Rels() []string { return rels(i.Controls) }

// Decode reads the item's JSON into a value.
func (i Item) Decode(into any) error { return json.Unmarshal(i.Data, into) }

// Resource is a response read into the model.
type Resource struct {
	Status   int
	Message  string
	Notes    []string
	Paging   *Paging
	Controls []Control
	Items    []Item
	// Problems are the server's reasons for refusing a request, when it gave them.
	Problems []Problem
	// Body is the whole response body.
	Body json.RawMessage
}

// Control finds a control by rel.
func (r *Resource) Control(rel string) (Control, bool) { return find(r.Controls, rel) }

// Rels lists the rels the resource offers.
func (r *Resource) Rels() []string { return rels(r.Controls) }

// Decode reads the first item's JSON into a value, which is the common case of a single resource.
func (r *Resource) Decode(into any) error {
	if len(r.Items) == 0 {
		return &NoDataError{}
	}
	return r.Items[0].Decode(into)
}

func find(controls []Control, rel string) (Control, bool) {
	for _, c := range controls {
		if c.Rel == rel {
			return c, true
		}
	}
	return Control{}, false
}

func rels(controls []Control) []string {
	out := make([]string, 0, len(controls))
	for _, c := range controls {
		out = append(out, c.Rel)
	}
	return out
}
