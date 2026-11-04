// Regression tests for behaviour gaps that were found by review and then fixed. Each test is named
// after its entry in ../../tmp/gap-tracking.md.
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
				"price":  map[string]any{"amount": "1", "currency": "USD", "note": "listing fee"},
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

// GAP-01: a control that says it requires a session is followed like any other. Nothing in the
// surfer reads `requires`; the server's 401 is what says a session is needed.
func TestGAP01_RequiresIsNotActedOn_TheServersAnswerDecides(t *testing.T) {
	var secretHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/secret" {
			secretHits.Add(1)
			writeGapJSON(w, 401, map[string]any{"status": 401, "message": "Sign in", "notes": []string{"missing: agent-session"}, "data": []any{}})
			return
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

	var refused *hypertext.UnexpectedStatusError
	if !errors.As(err, &refused) || refused.Status != 401 {
		t.Fatalf("err = %v, want the server's 401 as *UnexpectedStatusError", err)
	}
	if secretHits.Load() != 1 {
		t.Fatalf("GET /secret was called %d times; want the request sent once", secretHits.Load())
	}
}

// GAP-02: a control with a price is sent. A 402 from the server, not a field on the control, is what
// says payment is needed, because only the 402 carries the terms of that moment.
func TestGAP02_PriceIsNotActedOn_TheServersAnswerDecides(t *testing.T) {
	var postHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/paid" {
			postHits.Add(1)
			writeGapJSON(w, 402, map[string]any{"status": 402, "message": "Payment required", "data": []any{}})
			return
		}
		writeGapJSON(w, 200, gapHomeWithPaidAction())
	}))
	t.Cleanup(srv.Close)

	s, _ := surf.New(srv.URL, surf.WithDialects(agenttrader.New()))
	home, _ := s.Root(context.Background())
	_, err := home.Submit(context.Background(), "buy", surf.Values{"sku": "x"})

	var refused *hypertext.UnexpectedStatusError
	if !errors.As(err, &refused) || refused.Status != 402 {
		t.Fatalf("err = %v, want the server's 402 as *UnexpectedStatusError", err)
	}
	if postHits.Load() != 1 {
		t.Fatalf("POST /paid was called %d times; want the request sent once", postHits.Load())
	}
}

// GAP-03: a link's `type` is an extension too. Following it sends the request, and a response no
// dialect can read is reported as such.
func TestGAP03_TypeIsNotActedOn_AnUnreadableAnswerIsReported(t *testing.T) {
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

	var unsupported *hypertext.UnsupportedMediaTypeError
	if !errors.As(err, &unsupported) {
		t.Fatalf("err = %v, want *UnsupportedMediaTypeError", err)
	}
	if htmlHits.Load() != 1 {
		t.Fatalf("GET /human was called %d times; want once", htmlHits.Load())
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
	report, err := surf.NewWave(s).
		From("/listings").
		ThenItem(0).
		Go(context.Background())
	if err != nil {
		t.Fatalf("a wave should start from a bookmarked path: %v", err)
	}
	if len(report.Lines) != 2 || report.Lines[0].Status != 200 || !strings.HasSuffix(report.Lines[0].URL, "/listings") {
		t.Fatalf("report = %+v, want the first step to open /listings with a 200", report.Lines)
	}
}

// GAP-07: Page should expose the raw response body for formatter-style decode.
func TestGAP07_PageExposesRawResponseBody(t *testing.T) {
	srv := newServer(t)
	home := surftest.Root(t, surfer(t, srv))
	var envelope struct {
		Status  int `json:"status"`
		Actions []struct {
			Rel string `json:"rel"`
		} `json:"_actions"`
	}
	if err := json.Unmarshal(home.RawBody(), &envelope); err != nil {
		t.Fatalf("RawBody should be the whole JSON envelope: %v", err)
	}
	if envelope.Status != 200 || len(envelope.Actions) == 0 || envelope.Actions[0].Rel != "search-listings" {
		t.Fatalf("envelope = %+v, want status 200 and the search-listings action", envelope)
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
	if len(home.Resource.DuplicateRels()) == 0 {
		t.Fatal("want DuplicateRels to report duplicate rel names on the page")
	}
}

// A control that says it requires a session and also has a missing required field reports the
// missing field: nothing reads `requires` before the request.
func TestMissingField_IsReportedEvenWhenTheControlSaysItRequiresASession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeGapJSON(w, 200, map[string]any{
			"status": 200, "message": "OK", "data": []any{},
			"_actions": []any{
				map[string]any{
					"rel": "shortlist", "action": "/shortlist", "method": "POST",
					"requires": []string{"agent-session"},
					"fields":   []any{map[string]any{"name": "listingId", "required": true}},
				},
			},
		})
	}))
	t.Cleanup(srv.Close)

	s, _ := surf.New(srv.URL, surf.WithDialects(agenttrader.New()))
	home, err := s.Root(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, err = home.Submit(context.Background(), "shortlist", surf.Values{})
	var missing *hypertext.MissingInputError
	if !errors.As(err, &missing) {
		t.Fatalf("err = %v, want *MissingInputError", err)
	}
}
