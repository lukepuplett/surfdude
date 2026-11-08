// Package simple reads Surfdude's own media type, application/simple-hypertext+json, the one the
// C# client uses by default:
//
//	{"links":[{"rel":"registrations","href":"/registrations","method":"GET",
//	           "form":[{"name":"skip","isOptional":true}]}],
//	 "items":[{"links":[...]}]}
//
// A link with a form is a form control; a link without is a plain link. A missing method means GET.
package simple

import (
	"encoding/json"
	"mime"
	"net/http"

	"github.com/lukepuplett/surfdude/go/hypertext"
)

// MediaType is Surfdude's own media type.
const MediaType = "application/simple-hypertext+json"

// Dialect reads the simple hypertext media type.
type Dialect struct{}

// New returns the dialect.
func New() Dialect { return Dialect{} }

func (Dialect) Name() string     { return "simple" }
func (Dialect) Accept() string   { return MediaType }
func (Dialect) BodyType() string { return "application/json" }

func (Dialect) Matches(resp *http.Response, _ []byte) bool {
	mt, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	return err == nil && mt == MediaType
}

type document struct {
	Links []link     `json:"links"`
	Items []document `json:"items"`
}

type link struct {
	Rel         string  `json:"rel"`
	Description string  `json:"description"`
	Href        string  `json:"href"`
	Method      string  `json:"method"`
	IfMatch     string  `json:"ifMatch"`
	Form        []input `json:"form"`
}

type input struct {
	Name       string   `json:"name"`
	Label      string   `json:"label"`
	IsOptional bool     `json:"isOptional"`
	Options    []string `json:"options"`
	Value      any      `json:"value"`
	// ReadOnly marks a value the API fixed: it is shown and sent, and not for editing.
	ReadOnly bool `json:"readOnly"`
}

func (Dialect) Read(resp *http.Response, body []byte) (*hypertext.Resource, error) {
	var doc document
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}

	res := &hypertext.Resource{Status: resp.StatusCode, Message: http.StatusText(resp.StatusCode), Controls: controls(doc.Links), Body: body}
	for _, item := range doc.Items {
		raw, _ := json.Marshal(item)
		res.Items = append(res.Items, hypertext.Item{Controls: controls(item.Links), Data: raw})
	}

	return res, nil
}

func controls(links []link) []hypertext.Control {
	out := make([]hypertext.Control, 0, len(links))
	for _, l := range links {
		c := hypertext.Control{Rel: l.Rel, Href: l.Href, Method: l.Method, IfMatch: l.IfMatch, Form: len(l.Form) > 0}
		if l.Description != "" {
			desc, _ := json.Marshal(l.Description)
			c.Extensions = map[string]json.RawMessage{"description": desc}
		}
		for _, in := range l.Form {
			hin := hypertext.Input{Name: in.Name, Required: !in.IsOptional, Value: in.Value}
			schema := hypertext.Schema{}
			if in.Label != "" {
				schema["description"] = in.Label
			}
			if len(in.Options) > 0 {
				enum := make([]any, len(in.Options))
				for i, o := range in.Options {
					enum[i] = o
				}
				schema["enum"] = enum
			}
			if in.ReadOnly {
				schema["readOnly"] = true
			}
			if len(schema) > 0 {
				hin.Schema = schema
			}
			c.Inputs = append(c.Inputs, hin)
		}
		out = append(out, c)
	}
	return out
}
