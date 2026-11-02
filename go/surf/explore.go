package surf

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/lukepuplett/surfdude/go/hypertext"
)

// ExploreOptions control a crawl.
type ExploreOptions struct {
	// MaxPages stops the crawl. Zero means 200.
	MaxPages int
	// KnownRel, when set, is asked about every rel found. A rel it rejects is reported, so a
	// registry of rels can be checked against what the live API really sends.
	KnownRel func(rel string) bool
}

// Visit is one page the crawl read.
type Visit struct {
	URL     string
	Status  int
	Dialect string
	Rels    []string
	Via     string
}

// Finding is something the crawl thinks is wrong.
type Finding struct {
	Kind    string
	Where   string
	Message string
}

// ExploreReport is what a crawl found.
type ExploreReport struct {
	Visits   []Visit
	Skipped  []string
	Findings []Finding
}

// Explore crawls everything reachable from the front door by following links, and forms that can be
// sent as a GET with nothing required. It proves reachability: a route nothing links to is never
// visited. It also checks every control it meets, and reports dead links, responses no dialect can
// read, rels a registry does not know, and controls that are malformed.
//
// It never submits a form that changes anything, and it skips controls that need a token it does
// not hold, saying so in the report.
func (s *Surfer) Explore(ctx context.Context, opts ExploreOptions) (*ExploreReport, error) {
	max := opts.MaxPages
	if max == 0 {
		max = 200
	}

	report := &ExploreReport{}
	root, err := s.Root(ctx)
	if err != nil {
		return report, err
	}

	type queued struct {
		from *Page
		c    hypertext.Control
		via  string
	}
	seen := map[string]bool{key(root.URL): true}
	report.Visits = append(report.Visits, visit(root, "root"))
	queue := []queued{}

	enqueue := func(p *Page, where string, controls []hypertext.Control) {
		for _, c := range controls {
			s.checkControl(report, opts, p, where, c)
			if reason := s.skipReason(c); reason != "" {
				report.Skipped = append(report.Skipped, fmt.Sprintf("%s %s on %s: %s", c.HTTPMethod(), c.Rel, p.URL.Path, reason))
				continue
			}
			queue = append(queue, queued{from: p, c: c, via: c.Rel})
		}
	}

	collect := func(p *Page) {
		enqueue(p, p.URL.Path, p.Resource.Controls)
		for i, it := range p.Resource.Items {
			enqueue(p, fmt.Sprintf("%s item %d", p.URL.Path, i), it.Controls)
		}
	}
	collect(root)

	for len(queue) > 0 && len(report.Visits) < max {
		q := queue[0]
		queue = queue[1:]

		target, err := q.from.URL.Parse(q.c.Href)
		if err != nil {
			continue
		}
		if seen[key(target)] {
			continue
		}
		seen[key(target)] = true

		req, err := buildRequest(ctx, s.reg, q.from.URL, q.c, map[string]any{}, s.bearer())
		if err != nil {
			report.Findings = append(report.Findings, Finding{"bad-control", q.from.URL.Path, err.Error()})
			continue
		}
		page, err := s.do(req, fmt.Sprintf("follow %q", q.c.Rel), func(int) bool { return true }, Never())
		if err != nil {
			kind := "unreadable"
			var unsupported *hypertext.UnsupportedMediaTypeError
			if errors.As(err, &unsupported) {
				kind = "not-hypermedia"
			}
			report.Findings = append(report.Findings, Finding{kind, target.Path, err.Error()})
			continue
		}

		report.Visits = append(report.Visits, visit(page, q.via))
		if page.Status >= 400 {
			report.Findings = append(report.Findings, Finding{"dead-link", target.Path, fmt.Sprintf("%s was followed from %s and answered %d", q.c.Rel, q.from.URL.Path, page.Status)})
			continue
		}
		collect(page)
	}

	sort.Strings(report.Skipped)
	return report, nil
}

func key(u *url.URL) string { return u.Path + "?" + u.RawQuery }

func visit(p *Page, via string) Visit {
	rels := p.Resource.Rels()
	return Visit{URL: p.URL.RequestURI(), Status: p.Status, Dialect: p.Dialect, Rels: rels, Via: via}
}

func (s *Surfer) skipReason(c hypertext.Control) string {
	switch {
	case c.HTTPMethod() != "GET":
		return "changes state, not followed"
	case len(c.Requires) > 0 && s.bearer() == "":
		return "needs " + strings.Join(c.Requires, ", ") + " and no token was given"
	case len(c.RequiredInputs()) > 0:
		names := make([]string, 0)
		for _, in := range c.RequiredInputs() {
			names = append(names, in.Name)
		}
		return "needs values for " + strings.Join(names, ", ")
	case strings.Contains(c.Href, "{") && strings.Contains(c.Href, "}") && !strings.Contains(c.Href, "{?"):
		return "has a path template with no value"
	case c.Type != "" && !strings.Contains(c.Type, "json"):
		return "serves " + c.Type + ", not hypermedia"
	}
	return ""
}

// checkControl reports a control that is malformed or uses a rel the registry does not know.
func (s *Surfer) checkControl(r *ExploreReport, opts ExploreOptions, p *Page, where string, c hypertext.Control) {
	add := func(kind, msg string) { r.Findings = append(r.Findings, Finding{kind, where, msg}) }

	if c.Rel == "" {
		add("bad-control", "a control has no rel")
	} else if opts.KnownRel != nil && !opts.KnownRel(c.Rel) {
		add("unknown-rel", fmt.Sprintf("the rel %q is not in the registry", c.Rel))
	}
	if c.Href == "" {
		add("bad-control", fmt.Sprintf("%s has no address", c.Rel))
	}
	switch c.HTTPMethod() {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD":
	default:
		add("bad-control", fmt.Sprintf("%s has the method %q, which is not HTTP", c.Rel, c.Method))
	}
	if c.Form && len(c.Inputs) == 0 && c.HTTPMethod() != "GET" && c.HTTPMethod() != "DELETE" {
		// A form with no inputs is legal (an action that needs nothing), so this is advice only.
		return
	}
	for _, in := range c.Inputs {
		if in.Name == "" {
			add("bad-control", fmt.Sprintf("%s has an input with no name", c.Rel))
		}
		if t, ok := in.Schema["type"].(string); ok {
			switch t {
			case "string", "integer", "number", "boolean", "array", "object":
			default:
				add("bad-control", fmt.Sprintf("%s input %s has the unknown type %q", c.Rel, in.Name, t))
			}
		}
		if enum, ok := in.Schema["enum"].([]any); ok && len(enum) == 0 {
			add("bad-control", fmt.Sprintf("%s input %s has an empty enum", c.Rel, in.Name))
		}
	}
}
