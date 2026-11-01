// Package agenttrader_test holds journeys against a running AgentTrader. They know only the front
// door and the rels the server offers: no paths, no methods. Run them with the server's address:
//
//	AGENTTRADER_URL=http://localhost:8080 go test ./examples/agenttrader
//
// or let scripts/journey-agenttrader.sh start the server and run them. Without the address they are
// skipped. A journey whose rels the server does not offer yet is pending: it is skipped with the
// name of what to build next. Set SURF_RED=1 to run the pending ones and watch them fail.
package agenttrader_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/lukepuplett/surfdude/go/dialects/agenttrader"
	"github.com/lukepuplett/surfdude/go/surf"
	"github.com/lukepuplett/surfdude/go/surftest"
)

func root(t *testing.T) *surf.Surfer {
	t.Helper()
	url := os.Getenv("AGENTTRADER_URL")
	if url == "" {
		t.Skip("set AGENTTRADER_URL to a running AgentTrader to run the journeys")
	}
	return surftest.New(t, url, surf.WithDialects(agenttrader.New()))
}

// pending skips a journey whose next step is not built yet, saying what to build.
func pending(t *testing.T, p *surf.Page, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		if !p.Has(rel) && os.Getenv("SURF_RED") == "" {
			t.Skipf("pending: the front door does not offer %q yet. It offers: %s", rel, strings.Join(p.Rels(), ", "))
		}
	}
}

// Journey 0: the front door
//
//	Given a running AgentTrader
//	When a client opens the front door
//	Then it is read as the agenttrader dialect, with a self link and advice for the agent
//	And every control it offers is registered, followable or explained
func TestJourney0_TheFrontDoor(t *testing.T) {
	s := root(t)

	home := surftest.Root(t, s)

	if home.Dialect != "agenttrader" {
		t.Errorf("dialect = %q, want agenttrader", home.Dialect)
	}
	if !home.Has("self") {
		t.Errorf("the front door should link to itself. It offers: %v", home.Rels())
	}
	if len(home.Notes()) == 0 {
		t.Error("the front door should prime the agent with notes")
	}
	if got := home.Header.Get("Link"); !strings.Contains(got, `rel="profile"`) {
		t.Errorf("Link = %q, want the envelope profile so a client can choose its dialect", got)
	}
}

// Journey 0b: everything reachable is sound
//
//	When a client crawls every link from the front door
//	Then no link is dead and every response is readable
func TestJourney0_EverythingReachableIsSound(t *testing.T) {
	s := root(t)

	report, err := s.Explore(context.Background(), surf.ExploreOptions{})
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range report.Findings {
		t.Errorf("[%s] %s: %s", f.Kind, f.Where, f.Message)
	}
	if len(report.Visits) == 0 {
		t.Error("the crawl should reach at least the front door")
	}
}

// Journey 1: search the market
//
//	Given the front door offers a typed search form
//	When a client searches for a MINI near Crowborough
//	Then it gets a page of results, each with its own shortlist form, and a next link when there is more
func TestJourney1_SearchTheMarket(t *testing.T) {
	s := root(t)
	home := surftest.Root(t, s)
	pending(t, home, "search-listings")

	results := surftest.Follow(t, home, "search-listings", surf.With(surf.Values{"make": "MINI", "lat": 51.06, "lon": 0.16, "radiusMiles": 30}))

	if results.Resource.Paging == nil {
		t.Error("a list should say how it is paged")
	}
	if len(results.Resource.Items) > 0 {
		first, err := results.Item(0)
		if err != nil {
			t.Fatal(err)
		}
		if !first.Has("self") {
			t.Errorf("each result should link to itself. It offers: %v", first.Rels())
		}
	}
}

// Journey 2: sign in, then list a car
//
//	Given the front door offers sign-in
//	When an agent signs in with its agent key and password, then follows new-listing
//	Then it gets a form it can submit
func TestJourney2_SignInThenNewListing(t *testing.T) {
	s := root(t)
	home := surftest.Root(t, s)
	pending(t, home, "sign-in", "new-listing")
	t.Skip("needs a registered agent: register, then sign in with the key and password returned")
}
