package surf_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lukepuplett/surfdude/go/dialects/agenttrader"
	"github.com/lukepuplett/surfdude/go/hypertext"
	"github.com/lukepuplett/surfdude/go/surf"
)

// plainServer answers like a server that is not hypermedia at all (a default mux 404, a proxy's
// error page), plus one hypermedia refusal that carries a header a test may want to read.
func plainServer(t *testing.T) *surf.Surfer {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /plain404", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("nothing here"))
	})
	mux.HandleFunc("GET /text200", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello"))
	})
	mux.HandleFunc("GET /limited", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Add("Link", profile)
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"status":429,"message":"Slow down","notes":["Wait."],"data":[],"_links":[{"rel":"home","href":"/","method":"GET"}]}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	s, err := surf.New(srv.URL+"/", surf.WithDialects(agenttrader.New()))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Scenario: a response no dialect can read still gives a page that is safe to ask
//
//	Given a server that answers a missing route with plain text
//	When a journey opens it
//	Then the error says the server answered 404
//	And the page that comes with it can be asked for rels, notes and the body without a panic
func TestUnreadablePage_CanBeAskedWithoutPanicking(t *testing.T) {
	// Arrange
	s := plainServer(t)

	// Act
	page, err := s.Open(context.Background(), "/plain404")

	// Assert
	var unexpected *hypertext.UnexpectedStatusError
	if !errors.As(err, &unexpected) || unexpected.Status != 404 {
		t.Fatalf("Open returned %v, want *UnexpectedStatusError with status 404", err)
	}
	if page == nil {
		t.Fatal("the page that came with the error is nil; it carries the status and headers")
	}
	if page.Status != 404 {
		t.Errorf("Status = %d, want 404", page.Status)
	}
	if rels := page.Rels(); len(rels) != 0 {
		t.Errorf("Rels() = %v, want none", rels)
	}
	if page.Has("home") {
		t.Error("Has(home) = true for a page with no controls")
	}
	if notes := page.Notes(); len(notes) != 0 {
		t.Errorf("Notes() = %v, want none", notes)
	}
	if got := string(page.RawBody()); got != "nothing here" {
		t.Errorf("RawBody() = %q, want the body the server sent, so a test can see what came back", got)
	}
}

func TestUnreadablePage_FollowSaysWhatItOffers_NotAPanic(t *testing.T) {
	s := plainServer(t)
	page, _ := s.Open(context.Background(), "/plain404")

	_, followErr := page.Follow(context.Background(), "home")
	_, itemErr := page.Item(0)

	var missing *hypertext.RelNotFoundError
	if !errors.As(followErr, &missing) {
		t.Errorf("Follow on an unreadable page returned %v, want *RelNotFoundError", followErr)
	}
	if itemErr == nil {
		t.Error("Item(0) on an unreadable page should be an error, not an item")
	}
	if err := page.Read(&struct{}{}); err == nil {
		t.Error("Read on an unreadable page should be an error")
	}
}

// A 200 the dialects cannot read is a different error, and its page must be safe too.
func TestUnreadablePage_AnExpectedStatusNoDialectUnderstands(t *testing.T) {
	s := plainServer(t)

	page, err := s.Open(context.Background(), "/text200")

	var unsupported *hypertext.UnsupportedMediaTypeError
	if !errors.As(err, &unsupported) {
		t.Fatalf("Open returned %v, want *UnsupportedMediaTypeError", err)
	}
	if page == nil || len(page.Rels()) != 0 || string(page.RawBody()) != "hello" {
		t.Errorf("the page should be safe and keep the body, got %+v", page)
	}
}

// Scenario: a refused step keeps the headers and body the server sent
//
//	Given a server that answers 429 with Retry-After
//	When a journey takes the step without expecting it
//	Then the error carries the status, the headers and the body, so a test can read Retry-After
func TestUnexpectedStatusError_KeepsHeadersAndBody(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantHeader string
		wantBody   string
	}{
		{"a readable refusal", "/limited", 429, "30", "Slow down"},
		{"a response no dialect reads", "/plain404", 404, "", "nothing here"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			s := plainServer(t)

			// Act
			_, err := s.Open(context.Background(), tc.path)

			// Assert
			var unexpected *hypertext.UnexpectedStatusError
			if !errors.As(err, &unexpected) {
				t.Fatalf("returned %v, want *UnexpectedStatusError", err)
			}
			if unexpected.Status != tc.wantStatus {
				t.Errorf("Status = %d, want %d", unexpected.Status, tc.wantStatus)
			}
			if got := unexpected.Header.Get("Retry-After"); got != tc.wantHeader {
				t.Errorf("Header Retry-After = %q, want %q", got, tc.wantHeader)
			}
			if !strings.Contains(string(unexpected.Body), tc.wantBody) {
				t.Errorf("Body = %q, want it to contain %q", unexpected.Body, tc.wantBody)
			}
		})
	}
}
