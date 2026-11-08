package surftest_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lukepuplett/surfdude/go/dialects/agenttrader"
	"github.com/lukepuplett/surfdude/go/surf"
	"github.com/lukepuplett/surfdude/go/surftest"
)

// refusingAPI refuses register with 429 and offers verify-email on the refusal, as a well-behaved
// API does, and accepts register-ok with 202.
func refusingAPI(t *testing.T) *surf.Page {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Add("Link", `<https://agenttrader.net/profiles/envelope/v1>; rel="profile"`)
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `{"status":200,"message":"OK","data":[],"_actions":[`+
				`{"rel":"register","action":"/start","method":"POST","fields":[{"name":"email","required":true}]},`+
				`{"rel":"register-ok","action":"/ok","method":"POST","fields":[{"name":"email","required":true}]}]}`)
		case "/start":
			w.Header().Set("Retry-After", "90")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"status":429,"message":"Too soon","data":[{"field":"email","code":"too-soon","message":"Wait."}],`+
				`"_actions":[{"rel":"verify-email","action":"/verify","method":"POST","fields":[{"name":"code","required":true}]}]}`)
		default:
			w.WriteHeader(http.StatusAccepted)
			fmt.Fprint(w, `{"status":202,"message":"Accepted","data":[]}`)
		}
	}))
	t.Cleanup(srv.Close)
	s := surftest.New(t, srv.URL+"/", surf.WithDialects(agenttrader.New()))
	p, err := s.Root(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	return p
}

// Scenario: RefusedPage returns the refusal page, with its status, headers, problems and controls
func TestRefusedPage_ReturnsTheRefusalPage(t *testing.T) {
	home := refusingAPI(t)

	refused := surftest.RefusedPage(t, home, "register", surf.Values{"email": "me@example.test"}, 429)

	if refused.Status != 429 {
		t.Errorf("Status = %d, want 429", refused.Status)
	}
	if got := refused.Header.Get("Retry-After"); got != "90" {
		t.Errorf("Retry-After = %q, want 90, which is why the page is returned and not only the problems", got)
	}
	if !refused.Has("verify-email") {
		t.Errorf("the refusal page offers %v, want verify-email", refused.Rels())
	}
	if len(refused.Resource.Problems) != 1 || refused.Resource.Problems[0].Field != "email" {
		t.Errorf("problems = %+v, want the one the server reported", refused.Resource.Problems)
	}
}

// Scenario: RefusedPage fails the test when the server accepted what it should have refused
func TestRefusedPage_FailsWhenTheServerAccepts(t *testing.T) {
	home := refusingAPI(t)

	r := expectFailure(t, func(tb testing.TB) {
		surftest.RefusedPage(tb, home, "register-ok", surf.Values{"email": "me@example.test"}, 429)
	})

	if !r.failed {
		t.Fatal("the test passed although the server answered 202, not the refusal that was expected")
	}
}
