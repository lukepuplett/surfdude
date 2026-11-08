package surf_test

import (
	"context"
	"io"
	"testing"

	"github.com/lukepuplett/surfdude/go/surf"
)

// surf.Unchecked lets a journey or a probe send what the form forbids, to see whether the server
// refuses it too. Without it the form is honoured and the request is never sent.

var forbidden = surf.Values{"email": "nonsense", "bogus": "1"} // a bad address, and a field the form does not have

// Scenario: a value the form forbids is refused locally, and sent only by an unchecked step
func TestUnchecked_ReachesTheServerOnlyWhenAsked(t *testing.T) {
	s, hits := prepareServer(t)
	home, err := s.Root(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	afterRoot := hits.Load()

	if _, err := home.Submit(context.Background(), "register", forbidden); err == nil {
		t.Fatal("a checked step sent a value the form forbids")
	}
	if hits.Load() != afterRoot {
		t.Fatalf("a refused checked step still made %d request(s)", hits.Load()-afterRoot)
	}

	if _, err := home.Submit(context.Background(), "register", forbidden, surf.Unchecked()); err != nil {
		t.Fatalf("an unchecked step failed: %v", err)
	}
	if hits.Load() != afterRoot+1 {
		t.Fatalf("an unchecked step made %d request(s), want 1", hits.Load()-afterRoot)
	}
}

// Scenario: the unchecked request carries the values exactly as given
func TestUnchecked_PrepareCarriesTheValuesAsGiven(t *testing.T) {
	s, _ := prepareServer(t)
	home, err := s.Root(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	req, err := home.Prepare(context.Background(), "register", surf.With(forbidden), surf.Unchecked())

	if err != nil {
		t.Fatalf("Prepare refused an unchecked step: %v", err)
	}
	body, _ := req.GetBody()
	raw, _ := io.ReadAll(body)
	if string(raw) != `{"bogus":"1","email":"nonsense"}` {
		t.Fatalf("body = %s, want both values exactly as given", raw)
	}
}
