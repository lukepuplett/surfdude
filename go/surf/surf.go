// Package surf drives a hypermedia API the way a person drives a website: start at the front door,
// follow links, submit forms. A journey names only rels, never URLs or HTTP methods, so it passes
// only if the API really can be driven by its own hypertext.
//
//	s, _ := surf.New("http://localhost:8080", surf.WithDialects(agenttrader.New()))
//	home, _ := s.Root(ctx)
//	results, _ := home.Submit(ctx, "search-listings", surf.Values{"make": "MINI"})
//	first, _ := results.Item(0)
//	_, err := first.Submit(ctx, "shortlist", surf.Values{"note": "nice colour"})
//
// When a step cannot be taken, the error says what the page did offer, so an API can be fixed until
// the journey runs. That is the point: write the journey first, then make the API able to carry it.
package surf

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/lukepuplett/surfdude/go/dialect"
	"github.com/lukepuplett/surfdude/go/hypertext"
)

const maxBody = 4 << 20

// Surfer holds what every step shares: the client, the front door, the dialects it understands and
// an optional bearer token.
type Surfer struct {
	client *http.Client
	root   *url.URL
	reg    *dialect.Registry
	token  string
	agent  string
}

// Option configures a Surfer.
type Option func(*Surfer)

// WithHTTPClient sets the HTTP client, for example an httptest server's client.
func WithHTTPClient(c *http.Client) Option { return func(s *Surfer) { s.client = c } }

// WithDialects sets the dialects, tried in order. At least one is needed.
func WithDialects(ds ...dialect.Dialect) Option {
	return func(s *Surfer) { s.reg = dialect.NewRegistry(ds...) }
}

// WithBearer sets the access token sent as Authorization: Bearer.
func WithBearer(token string) Option { return func(s *Surfer) { s.token = token } }

// New builds a Surfer for a front door URL.
func New(root string, opts ...Option) (*Surfer, error) {
	u, err := url.Parse(root)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("Unable to start. %q is not an absolute URL.", root)
	}

	s := &Surfer{client: &http.Client{Timeout: 30 * time.Second}, root: u, agent: "surfdude-go"}
	for _, o := range opts {
		o(s)
	}
	if s.reg == nil || len(s.reg.Names()) == 0 {
		return nil, fmt.Errorf("Unable to start. A Surfer needs at least one dialect: use surf.WithDialects.")
	}

	return s, nil
}

// SetBearer changes the access token, for example after signing in.
func (s *Surfer) SetBearer(token string) { s.token = token }

// Page is where the journey has got to: a response read into the neutral model.
type Page struct {
	s        *Surfer
	URL      *url.URL
	Status   int
	Header   http.Header
	Dialect  string
	Resource *hypertext.Resource
}

// Root fetches the front door.
func (s *Surfer) Root(ctx context.Context) (*Page, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.root.String(), nil)
	if err != nil {
		return nil, err
	}
	s.decorate(req)

	return s.do(req, "open the front door", nil)
}

func (s *Surfer) decorate(req *http.Request) {
	if accept := s.reg.Accept(); accept != "" {
		req.Header.Set("Accept", accept)
	}
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	req.Header.Set("User-Agent", s.agent)
}

// do sends a request and reads the response. expected is nil for "any 2xx".
func (s *Surfer) do(req *http.Request, step string, expected func(int) bool) (*Page, error) {
	if expected == nil {
		expected = func(code int) bool { return code >= 200 && code < 300 }
	}
	req.Header.Set("User-Agent", s.agent)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Unable to %s. The request failed: %w", step, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("Unable to %s. The response could not be read: %w", step, err)
	}

	page := &Page{s: s, URL: req.URL, Status: resp.StatusCode, Header: resp.Header}

	d, ok := s.reg.Choose(resp, body)
	if !ok {
		if !expected(resp.StatusCode) {
			return page, &hypertext.UnexpectedStatusError{Step: step, Status: resp.StatusCode}
		}
		return page, s.reg.Unsupported(resp)
	}
	page.Dialect = d.Name()

	res, err := d.Read(resp, body)
	if err != nil {
		return page, fmt.Errorf("Unable to %s. The %s dialect could not read the response: %w", step, d.Name(), err)
	}
	page.Resource = res

	if !expected(resp.StatusCode) {
		return page, &hypertext.UnexpectedStatusError{Step: step, Status: resp.StatusCode, Message: res.Message, Resource: res}
	}

	return page, nil
}

// Step options.
type stepConfig struct {
	values Values
	expect func(int) bool
}

// Step changes how one step is taken.
type Step func(*stepConfig)

// With gives the values to send to a form.
func With(v Values) Step { return func(c *stepConfig) { c.values = v } }

// Expect lists the statuses that count as success. Without it, any 2xx does. A journey that tests a
// refusal uses it, for example surf.Expect(422).
func Expect(codes ...int) Step {
	return func(c *stepConfig) {
		c.expect = func(got int) bool {
			for _, code := range codes {
				if got == code {
					return true
				}
			}
			return false
		}
	}
}

func config(opts []Step) stepConfig {
	var c stepConfig
	for _, o := range opts {
		o(&c)
	}
	return c
}

// Rels lists the rels the page offers.
func (p *Page) Rels() []string { return p.Resource.Rels() }

// Has says whether the page offers a control with the rel.
func (p *Page) Has(rel string) bool { _, ok := p.Resource.Control(rel); return ok }

// Control returns the control with the rel.
func (p *Page) Control(rel string) (hypertext.Control, bool) { return p.Resource.Control(rel) }

// Notes are the advice the server put on the response.
func (p *Page) Notes() []string { return p.Resource.Notes }

// Read decodes the first item's data into a value.
func (p *Page) Read(into any) error { return p.Resource.Decode(into) }

// Follow takes the control with the given rel, sending any values it needs.
func (p *Page) Follow(ctx context.Context, rel string, opts ...Step) (*Page, error) {
	return p.take(ctx, p.Resource.Controls, "This page", rel, opts)
}

// Submit is Follow with the values to send.
func (p *Page) Submit(ctx context.Context, rel string, v Values, opts ...Step) (*Page, error) {
	return p.take(ctx, p.Resource.Controls, "This page", rel, append(opts, With(v)))
}

// ItemPage is one member of a list, with controls of its own.
type ItemPage struct {
	page  *Page
	index int
	item  hypertext.Item
}

// Item selects a member of the list.
func (p *Page) Item(i int) (*ItemPage, error) {
	if i < 0 || i >= len(p.Resource.Items) {
		return nil, fmt.Errorf("Unable to select item %d. The page has %d.", i, len(p.Resource.Items))
	}
	return &ItemPage{page: p, index: i, item: p.Resource.Items[i]}, nil
}

// Rels lists the rels the item offers.
func (i *ItemPage) Rels() []string { return i.item.Rels() }

// Has says whether the item offers a control with the rel.
func (i *ItemPage) Has(rel string) bool { _, ok := i.item.Control(rel); return ok }

// Notes are the advice the server put on this item.
func (i *ItemPage) Notes() []string { return i.item.Notes }

// Read decodes the item's data into a value.
func (i *ItemPage) Read(into any) error { return i.item.Decode(into) }

// Follow takes one of the item's controls.
func (i *ItemPage) Follow(ctx context.Context, rel string, opts ...Step) (*Page, error) {
	return i.page.take(ctx, i.item.Controls, fmt.Sprintf("Item %d", i.index), rel, opts)
}

// Submit is Follow with the values to send.
func (i *ItemPage) Submit(ctx context.Context, rel string, v Values, opts ...Step) (*Page, error) {
	return i.page.take(ctx, i.item.Controls, fmt.Sprintf("Item %d", i.index), rel, append(opts, With(v)))
}

func (p *Page) take(ctx context.Context, controls []hypertext.Control, where, rel string, opts []Step) (*Page, error) {
	var c hypertext.Control
	found := false
	for _, candidate := range controls {
		if candidate.Rel == rel {
			c, found = candidate, true
			break
		}
	}
	if !found {
		offered := make([]string, len(controls))
		for i, cc := range controls {
			offered[i] = cc.Rel
		}
		return nil, &hypertext.RelNotFoundError{Rel: rel, Where: where, Offered: offered}
	}

	cfg := config(opts)
	vals, err := merge(c, cfg.values)
	if err != nil {
		return nil, err
	}

	req, err := buildRequest(ctx, p.s.reg, p.URL, c, vals, p.s.token)
	if err != nil {
		return nil, err
	}

	return p.s.do(req, fmt.Sprintf("follow %q", rel), cfg.expect)
}
