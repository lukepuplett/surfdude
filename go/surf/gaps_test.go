//go:build gap

// Red repro tests for behaviour gaps (see ../../tmp/gap-tracking.md).
// Run: go test -tags=gap ./go/surf/...
package surf_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lukepuplett/surfdude/go/dialects/agenttrader"
	"github.com/lukepuplett/surfdude/go/hypertext"
	"github.com/lukepuplett/surfdude/go/surf"
	"github.com/lukepuplett/surfdude/go/surftest"
)

func writeGapJSON(w http.ResponseWriter, status int, body map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Add("Link", profile)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func gapHomeWithSecret() map[string]any {
	return map[string]any{
		"status": 200, "message": "OK", "data": []any{map[string]any{"name": "Fake"}},
		"_links": []any{
			map[string]any{"rel": "self", "href": "/", "method": "GET"},
			map[string]any{"rel": "secret", "href": "/secret", "method": "GET", "requires": []string{"agent-session"}},
		},
	}
}

func gapHomeWithPaidAction() map[string]any {
	return map[string]any{
		"status": 200, "message": "OK", "data": []any{},
		"_actions": []any{
			map[string]any{
				"rel": "buy", "action": "/paid", "method": "POST", "enctype": "application/json",
				"price": map[string]any{"amount": "1", "currency": "USD", "note": "listing fee"},
				"fields": []any{map[string]any{"name": "sku", "required": true, "schema": map[string]any{"type": "string"}}},
			},
		},
	}
}

func gapHomeWithHumanPage() map[string]any {
	return map[string]any{
		"status": 200, "message": "OK", "data": []any{},
		"_links": []any{
			map[string]any{"rel": "self", "href": "/", "method": "GET"},
			map[string]any{"rel": "human-page", "href": "/human", "method": "GET", "type": "text/html"},
		},
	}
}

func gapHomeWithEmailForm() map[string]any {
	return map[string]any{
		"status": 200, "message": "OK", "data": []any{},
		"_actions": []any{
			map[string]any{
				"rel": "register", "action": "/auth/start", "method": "POST", "enctype": "application/json",
				"fields": []any{
					map[string]any{"name": "email", "required": true, "schema": map[string]any{"type": "string", "format": "email"}},
				},
			},
		},
	}
}

func gapHomeDuplicateRel() map[string]any {
	return map[string]any{
		"status": 200, "message": "OK", "data": []any{},
		"_links": []any{
			map[string]any{"rel": "self", "href": "/a", "method": "GET"},
			map[string]any{"rel": "self", "href": "/b", "method": "GET"},
		},
	}
}

// GAP-01: Follow a control with requires agent-session without a token must not hit the server.
func TestGAP01_RequiresPreflightBeforeHTTP(t *testing.T) {
	var secretHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/secret" {
			secretHits.Add(1)
		}
		writeGapJSON(w, 200, gapHomeWithSecret())
	}))
	t.Cleanup(srv.Close)

	s, _ := surf.New(srv.URL, surf.WithDialects(agenttrader.New()))
	home, err := s.Root(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, err = home.Follow(context.Background(), "secret")
	if err == nil {
		t.Fatal("want an error before or instead of an unauthenticated GET /secret")
	}
	if !strings.Contains(err.Error(), "agent-session") {
		t.Fatalf("err = %v, want it to name agent-session", err)
	}
	if secretHits.Load() != 0 {
		t.Fatalf("GET /secret was called %d times; want preflight without HTTP", secretHits.Load())
	}
}

// GAP-02: Submit on a control with price must fail before POST.
func TestGAP02_PricePreflightBeforePOST(t *testing.T) {
	var postHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/paid" {
			postHits.Add(1)
		}
		writeGapJSON(w, 200, gapHomeWithPaidAction())
	}))
	t.Cleanup(srv.Close)

	s, _ := surf.New(srv.URL, surf.WithDialects(agenttrader.New()))
	home, _ := s.Root(context.Background())
	_, err := home.Submit(context.Background(), "buy", surf.Values{"sku": "x"})
	if err == nil {
		t.Fatal("want an error about payment before POST")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "pay") {
		t.Fatalf("err = %v, want payment called out", err)
	}
	if postHits.Load() != 0 {
		t.Fatalf("POST /paid was called; want preflight")
	}
}

// GAP-03: Follow on a link whose type is not hypermedia must fail before GET.
func TestGAP03_NonHypermediaTypePreflight(t *testing.T) {
	var htmlHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/human" {
			htmlHits.Add(1)
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html></html>"))
			return
		}
		writeGapJSON(w, 200, gapHomeWithHumanPage())
	}))
	t.Cleanup(srv.Close)

	s, _ := surf.New(srv.URL, surf.WithDialects(agenttrader.New()))
	home, _ := s.Root(context.Background())
	_, err := home.Follow(context.Background(), "human-page")
	if err == nil {
		t.Fatal("want an error before fetching non-hypermedia")
	}
	if htmlHits.Load() != 0 {
		t.Fatalf("GET /human was called; want preflight")
	}
}

// GAP-04: Read on a list page with multiple items must require Item(i).
func TestGAP04_ReadOnListPageMustSelectItem(t *testing.T) {
	srv := newServer(t)
	s := surfer(t, srv)
	home := surftest.Root(t, s)
	results, err := home.Submit(context.Background(), "search-listings", surf.Values{"make": "MINI"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Resource.Items) < 2 {
		t.Fatalf("need multiple items, got %d", len(results.Resource.Items))
	}
	var got struct {
		ID string `json:"id"`
	}
	err = results.Read(&got)
	if err == nil {
		t.Fatal("want an error when calling Read on a multi-item list page")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "item") {
		t.Fatalf("err = %v, want guidance to select an item", err)
	}
}

// GAP-05: After an expected 422, Read must not silently decode problem rows as a business DTO.
func TestGAP05_ReadAfter422ShouldNotDecodeAsBusinessDTO(t *testing.T) {
	srv := newServer(t)
	home := surftest.Root(t, surfer(t, srv))
	page, err := home.Submit(context.Background(), "create-listing", surf.Values{"reg": ""}, surf.Expect(422))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Resource.Problems) == 0 {
		t.Fatal("need problems on the 422 page")
	}
	var business struct {
		ID string `json:"id"`
	}
	err = page.Read(&business)
	if err == nil && business.ID != "" {
		t.Fatalf("Read decoded %+v from problem payload; want refusal or NoDataError", business)
	}
	var nd *hypertext.NoDataError
	if err == nil || !errors.As(err, &nd) {
		t.Fatalf("err = %v, want NoDataError when Read on problem-only resource", err)
	}
}

// GAP-06: Wave should start from an arbitrary URL, not only FromRoot.
func TestGAP06_WaveFromArbitraryURL(t *testing.T) {
	srv := newServer(t)
	s := surfer(t, srv)
	w := surf.NewWave(s)
	if _, ok := any(w).(interface{ From(string) *surf.Wave }); !ok {
		t.Fatal("Wave has no From(url); want bookmark entry like C#")
	}
}

// GAP-07: Page should expose the raw response body for formatter-style decode.
func TestGAP07_PageExposesRawResponseBody(t *testing.T) {
	srv := newServer(t)
	home := surftest.Root(t, surfer(t, srv))
	if home.Resource == nil || len(home.Resource.Body) == 0 {
		t.Fatal("resource should carry body")
	}
	if _, ok := any(home).(interface{ RawBody() []byte }); !ok {
		t.Fatal("Page has no RawBody(); want public access to response bytes")
	}
}

// GAP-08: Wave chain with Expect(422) should succeed and leave problems on the page.
func TestGAP08_WaveExpect422(t *testing.T) {
	srv := newServer(t)
	s := surfer(t, srv)
	report, err := surf.NewWave(s).
		FromRoot().
		ThenSubmit("create-listing", surf.Values{"reg": ""}, surf.Expect(422)).
		Go(context.Background())
	if err != nil {
		t.Fatalf("wave with Expect(422) should succeed: %v", err)
	}
	if len(report.Lines) != 2 {
		t.Fatalf("report = %+v, want root + submit", report.Lines)
	}
}

// GAP-09: format: email in schema should be validated client-side.
func TestGAP09_SchemaFormatValidated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeGapJSON(w, 200, gapHomeWithEmailForm())
	}))
	t.Cleanup(srv.Close)

	s, _ := surf.New(srv.URL, surf.WithDialects(agenttrader.New()))
	home, _ := s.Root(context.Background())
	_, err := home.Submit(context.Background(), "register", surf.Values{"email": "not-an-email"})
	var inv *hypertext.InvalidInputError
	if !errors.As(err, &inv) {
		t.Fatalf("err = %v, want InvalidInputError for bad email format", err)
	}
}

// GAP-10: Duplicate rel on one resource should be reported when reading.
func TestGAP10_DuplicateRelReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeGapJSON(w, 200, gapHomeDuplicateRel())
	}))
	t.Cleanup(srv.Close)

	s, _ := surf.New(srv.URL, surf.WithDialects(agenttrader.New()))
	home, err := s.Root(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if home.Resource == nil {
		t.Fatal("need resource")
	}
	// Ideal: home.Resource.DuplicateRels() returns ["self"] when two controls share a rel.
	type duplicateReporter interface {
		DuplicateRels() []string
	}
	rep, ok := any(home.Resource).(duplicateReporter)
	if !ok {
		t.Fatal("Resource has no DuplicateRels(); want duplicate rel reporting")
	}
	if len(rep.DuplicateRels()) == 0 {
		t.Fatal("want DuplicateRels to report duplicate rel names on the page")
	}
}
