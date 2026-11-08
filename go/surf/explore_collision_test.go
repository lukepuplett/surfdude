package surf_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lukepuplett/surfdude/go/dialects/agenttrader"
	"github.com/lukepuplett/surfdude/go/surf"
)

// A value fixed in the href and an input with the same name offer one value as both fixed and
// editable (docs/model.md, Fixed and editable values). The explorer reports it as a problem.

func exploreAPI(t *testing.T, actions string) *surf.Surfer {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Add("Link", `<https://agenttrader.net/profiles/envelope/v1>; rel="profile"`)
		if r.URL.Path != "/" { // a followed link lands on a page with nothing more to offer
			fmt.Fprint(w, `{"status":200,"message":"OK","data":[]}`)
			return
		}
		fmt.Fprintf(w, `{"status":200,"message":"OK","data":[],"_actions":[%s]}`, actions)
	}))
	t.Cleanup(srv.Close)
	s, err := surf.New(srv.URL, surf.WithDialects(agenttrader.New()))
	if err != nil {
		t.Fatal(err)
	}

	return s
}

// Scenario: a control whose href fixes a parameter that one of its inputs repeats is reported
func TestExplore_ReportsAFieldThatRepeatsAFixedParameter(t *testing.T) {
	s := exploreAPI(t, `{"rel":"mailbox","action":"/box?to=a%40b.test","method":"GET","fields":[{"name":"to"},{"name":"from"}]}`)

	report, err := s.Explore(context.Background(), surf.ExploreOptions{})

	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, f := range report.Findings {
		found = append(found, f.Kind+": "+f.Message)
	}
	joined := strings.Join(found, "\n")
	if !strings.Contains(joined, `bad-control: mailbox: field "to" repeats a parameter fixed in the href`) {
		t.Fatalf("findings = %q, want the repeated parameter reported", found)
	}
	if strings.Contains(joined, `"from"`) {
		t.Errorf("a field that does not repeat anything was reported: %q", found)
	}
}

// Scenario: nothing is reported when the names differ, or when the value is a template, or there is none
func TestExplore_ReportsNothingWhenNothingRepeats(t *testing.T) {
	for name, actions := range map[string]string{
		"different names":      `{"rel":"mailbox","action":"/box?to=a%40b.test","method":"GET","fields":[{"name":"from"}]}`,
		"a template":           `{"rel":"mailbox","action":"/box{?to}","method":"GET","fields":[{"name":"to"}]}`,
		"no query in the href": `{"rel":"mailbox","action":"/box","method":"GET","fields":[{"name":"to"}]}`,
	} {
		s := exploreAPI(t, actions)

		report, err := s.Explore(context.Background(), surf.ExploreOptions{})

		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, f := range report.Findings {
			if strings.Contains(f.Message, "repeats a parameter") {
				t.Errorf("%s: unexpected finding %q", name, f.Message)
			}
		}
	}
}
