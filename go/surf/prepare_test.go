package surf_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/lukepuplett/surfdude/go/dialects/agenttrader"
	"github.com/lukepuplett/surfdude/go/hypertext"
	"github.com/lukepuplett/surfdude/go/surf"
)

// Prepare builds the request a step would send and sends nothing. The CLI's --dry-run uses it, and
// so can a test that wants to look at a request without making the call.

func prepareServer(t *testing.T) (*surf.Surfer, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Add("Link", `<https://agenttrader.net/profiles/envelope/v1>; rel="profile"`)
		_, _ = io.WriteString(w, `{"status":200,"message":"OK","data":[],"_actions":[`+
			`{"rel":"register","action":"/auth/email/start","method":"POST","fields":[{"name":"email","required":true,"schema":{"type":"string","format":"email"}}]}]}`)
	}))
	t.Cleanup(srv.Close)
	s, err := surf.New(srv.URL, surf.WithDialects(agenttrader.New()), surf.WithBearer("tok-123"))
	if err != nil {
		t.Fatal(err)
	}

	return s, &hits
}

// Scenario: Prepare gives the request a Submit would send, and sends nothing
func TestPrepare_BuildsTheRequestWithoutSendingIt(t *testing.T) {
	s, hits := prepareServer(t)
	home, err := s.Root(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	before := hits.Load()

	req, err := home.Prepare(context.Background(), "register", surf.With(surf.Values{"email": "me@example.test"}))

	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	if req == nil {
		t.Fatal("Prepare returned no request")
	}
	if req.Method != http.MethodPost || req.URL.Path != "/auth/email/start" {
		t.Fatalf("request = %s %s, want POST /auth/email/start", req.Method, req.URL.Path)
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer tok-123" {
		t.Fatalf("Authorization = %q, want the surfer's bearer token, as Submit would send", got)
	}
	body, err := req.GetBody()
	if err != nil {
		t.Fatalf("the request body cannot be read again: %v", err)
	}
	raw, _ := io.ReadAll(body)
	if string(raw) != `{"email":"me@example.test"}` {
		t.Fatalf("body = %s, want {\"email\":\"me@example.test\"}", raw)
	}
	if hits.Load() != before {
		t.Fatalf("Prepare made %d request(s) to the server, want none", hits.Load()-before)
	}
}

// Scenario: Prepare refuses what Submit would refuse, with the same errors
func TestPrepare_RefusesWhatSubmitRefuses(t *testing.T) {
	s, _ := prepareServer(t)
	home, err := s.Root(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	_, noRel := home.Prepare(context.Background(), "nope")
	_, missing := home.Prepare(context.Background(), "register")
	_, extra := home.Prepare(context.Background(), "register", surf.With(surf.Values{"email": "a@b.test", "colour": "red"}))
	_, bad := home.Prepare(context.Background(), "register", surf.With(surf.Values{"email": "not an address"}))

	var relErr *hypertext.RelNotFoundError
	var missErr *hypertext.MissingInputError
	var extraErr *hypertext.UnexpectedInputError
	var badErr *hypertext.InvalidInputError
	if !errors.As(noRel, &relErr) || !errors.As(missing, &missErr) || !errors.As(extra, &extraErr) || !errors.As(bad, &badErr) {
		t.Fatalf("errors = %v | %v | %v | %v, want rel-not-found, missing, unexpected and invalid input", noRel, missing, extra, bad)
	}
}
