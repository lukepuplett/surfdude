package surf_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/lukepuplett/surfdude/go/dialects/agenttrader"
	"github.com/lukepuplett/surfdude/go/surf"
)

// Feature: Challenge handlers
//
//	Scenario: listing-weeks 402 triggers sandbox credit and retries publish
func TestWithChallenges_ListingWeeksRetriesPublish(t *testing.T) {
	var publishHits atomic.Int32
	var credited atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Add("Link", `<`+agenttrader.ProfileURI+`>; rel="profile"`)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 200, "message": "OK", "data": []any{map[string]any{"name": "test"}},
				"_actions": []map[string]any{{
					"rel": "publish", "action": "/listings/lst_1/publish", "method": "POST",
					"enctype": "application/json",
					"fields":  []map[string]any{{"name": "weeks", "required": true, "schema": map[string]any{"type": "integer"}}},
				}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/sandbox/listing-advert-weeks":
			credited.Store(true)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": 200, "message": "OK", "data": []map[string]any{{"balance": 4}}})
		case r.Method == http.MethodPost && r.URL.Path == "/listings/lst_1/publish":
			n := publishHits.Add(1)
			if n == 1 && !credited.Load() {
				w.WriteHeader(http.StatusPaymentRequired)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"status": 402, "message": "Unable to publish the listing. Short of weeks.", "missing": "listing-weeks",
					"data":   []map[string]any{{"field": "weeks", "message": "need more"}},
					"_actions": []map[string]any{{
						"rel": "sandbox-listing-advert-credit", "action": "/sandbox/listing-advert-weeks", "method": "POST",
						"fields": []map[string]any{{"name": "weeks", "required": true}},
					}},
				})
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": 200, "message": "OK", "data": []map[string]any{{"state": "live"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	creditWeeks := surf.Challenge{
		Name:  "listing-weeks",
		Match: agenttrader.MatchCapabilityGap(http.StatusPaymentRequired, "listing-weeks"),
		Handle: func(ctx context.Context, cc surf.ChallengeContext) (bool, error) {
			if !cc.Page.Has("sandbox-listing-advert-credit") {
				return false, nil
			}
			_, err := cc.Page.Submit(ctx, "sandbox-listing-advert-credit", surf.Values{"weeks": 4})
			return err == nil, err
		},
	}

	s, err := surf.New(srv.URL, surf.WithDialects(agenttrader.New()), surf.WithChallenges(creditWeeks))
	if err != nil {
		t.Fatal(err)
	}
	home, err := s.Root(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	page, err := home.Submit(context.Background(), "publish", surf.Values{"weeks": 2})
	if err != nil {
		t.Fatalf("publish after challenge: %v", err)
	}
	if page.Status != http.StatusOK {
		t.Fatalf("status = %d, want 200", page.Status)
	}
	if publishHits.Load() != 2 {
		t.Fatalf("publish called %d times, want 2 (refused then retry)", publishHits.Load())
	}
	if !credited.Load() {
		t.Fatal("sandbox credit was not called")
	}
}

// Scenario: pure x402 402 does not match listing-weeks handler
func TestMatchCapabilityGap_IgnoresX402Body(t *testing.T) {
	body := []byte(`{"x402Version":1,"accepts":[{"scheme":"exact"}]}`)
	if agenttrader.MatchCapabilityGap(http.StatusPaymentRequired, "listing-weeks")(402, nil, body) {
		t.Fatal("x402 body should not match envelope capability gap")
	}
}
