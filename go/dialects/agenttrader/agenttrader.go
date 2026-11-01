// Package agenttrader reads the AgentTrader response envelope:
//
//	{"status":200,"message":"OK","notes":[...],"data":[...],"paging":{...},
//	 "_links":[{"rel","href","method","title","type","requires"}],
//	 "_actions":[{"rel","title","action","method","enctype","price","requires","ifMatch",
//	              "fields":[{"name","required","value","schema"}]}]}
//
// Items in data can carry their own _links, _actions and notes. Every response names the envelope
// in a Link header (rel="profile"), which is how the dialect recognises it without sniffing.
package agenttrader

import (
	"encoding/json"
	"mime"
	"net/http"
	"strings"

	"github.com/lukepuplett/surfdude/go/hypertext"
)

// ProfileURI names the envelope. The server sends it in a Link header with rel="profile".
const ProfileURI = "https://agenttrader.net/profiles/envelope/v1"

// Dialect reads the AgentTrader envelope.
type Dialect struct{}

// New returns the dialect.
func New() Dialect { return Dialect{} }

func (Dialect) Name() string     { return "agenttrader" }
func (Dialect) Accept() string   { return "application/json" }
func (Dialect) BodyType() string { return "application/json" }

// Matches recognises the envelope by its profile link, and failing that by its shape: a JSON object
// with a numeric status and a data list.
func (Dialect) Matches(resp *http.Response, body []byte) bool {
	mt, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || (mt != "application/json" && !strings.HasSuffix(mt, "+json")) {
		return false
	}
	for _, l := range resp.Header.Values("Link") {
		if strings.Contains(l, "<"+ProfileURI+">") && strings.Contains(l, `rel="profile"`) {
			return true
		}
	}

	var probe struct {
		Status *int              `json:"status"`
		Data   []json.RawMessage `json:"data"`
	}
	return json.Unmarshal(body, &probe) == nil && probe.Status != nil && probe.Data != nil
}

type envelope struct {
	Status  int               `json:"status"`
	Message string            `json:"message"`
	Notes   []string          `json:"notes"`
	Data    []json.RawMessage `json:"data"`
	Paging  *struct {
		Skip  int  `json:"skip"`
		Take  int  `json:"take"`
		Count int  `json:"count"`
		Total *int `json:"total"`
	} `json:"paging"`
	Links   []link   `json:"_links"`
	Actions []action `json:"_actions"`
}

type controls struct {
	Links   []link   `json:"_links"`
	Actions []action `json:"_actions"`
	Notes   []string `json:"notes"`
}

type link struct {
	Rel      string   `json:"rel"`
	Href     string   `json:"href"`
	Method   string   `json:"method"`
	Title    string   `json:"title"`
	Type     string   `json:"type"`
	Requires []string `json:"requires"`
}

type action struct {
	Rel      string   `json:"rel"`
	Title    string   `json:"title"`
	Action   string   `json:"action"`
	Method   string   `json:"method"`
	Enctype  string   `json:"enctype"`
	Requires []string `json:"requires"`
	IfMatch  string   `json:"ifMatch"`
	Price    *struct {
		Amount   string `json:"amount"`
		Currency string `json:"currency"`
		Note     string `json:"note"`
	} `json:"price"`
	Fields []field `json:"fields"`
}

type field struct {
	Name     string         `json:"name"`
	Required bool           `json:"required"`
	Value    any            `json:"value"`
	Schema   map[string]any `json:"schema"`
}

func (Dialect) Read(resp *http.Response, body []byte) (*hypertext.Resource, error) {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}

	res := &hypertext.Resource{
		Status: env.Status, Message: env.Message, Notes: env.Notes,
		Controls: toControls(env.Links, env.Actions), Body: body,
	}
	if res.Status == 0 {
		res.Status = resp.StatusCode
	}
	if env.Paging != nil {
		res.Paging = &hypertext.Paging{Skip: env.Paging.Skip, Take: env.Paging.Take, Count: env.Paging.Count, Total: env.Paging.Total}
	}

	for _, raw := range env.Data {
		var c controls
		_ = json.Unmarshal(raw, &c) // an item that is not an object simply has no controls
		res.Items = append(res.Items, hypertext.Item{Controls: toControls(c.Links, c.Actions), Notes: c.Notes, Data: raw})
	}

	// An error response lists what was wrong in data: [{field, code, message}].
	if res.Status >= 400 {
		for _, raw := range env.Data {
			var p struct {
				Field   string `json:"field"`
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			if json.Unmarshal(raw, &p) == nil && p.Message != "" {
				res.Problems = append(res.Problems, hypertext.Problem{Field: p.Field, Code: p.Code, Message: p.Message})
			}
		}
	}

	return res, nil
}

func toControls(links []link, actions []action) []hypertext.Control {
	out := make([]hypertext.Control, 0, len(links)+len(actions))
	for _, l := range links {
		out = append(out, hypertext.Control{Rel: l.Rel, Title: l.Title, Href: l.Href, Method: l.Method, Type: l.Type, Requires: l.Requires})
	}
	for _, a := range actions {
		c := hypertext.Control{
			Rel: a.Rel, Title: a.Title, Href: a.Action, Method: a.Method, Enctype: a.Enctype,
			Form: true, Requires: a.Requires, IfMatch: a.IfMatch,
		}
		if a.Price != nil {
			c.Price = &hypertext.Price{Amount: a.Price.Amount, Currency: a.Price.Currency, Note: a.Price.Note}
		}
		for _, f := range a.Fields {
			c.Inputs = append(c.Inputs, hypertext.Input{Name: f.Name, Required: f.Required, Value: f.Value, Schema: f.Schema})
		}
		out = append(out, c)
	}
	return out
}
