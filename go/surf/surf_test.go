package surf_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/lukepuplett/surfdude/go/dialects/agenttrader"
	"github.com/lukepuplett/surfdude/go/hypertext"
	"github.com/lukepuplett/surfdude/go/surf"
	"github.com/lukepuplett/surfdude/go/surftest"
)

// server is a small fake in the AgentTrader envelope. It records what it was sent, so a test can
// check that a journey which only knew rels produced the right requests.
type server struct {
	*httptest.Server
	mu      sync.Mutex
	queries []string
	bodies  map[string]string
	headers map[string]http.Header
}

const profile = `<https://agenttrader.net/profiles/envelope/v1>; rel="profile"`

func newServer(t *testing.T) *server {
	t.Helper()
	s := &server{bodies: map[string]string{}, headers: map[string]http.Header{}}
	mux := http.NewServeMux()

	write := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Add("Link", profile)
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(v)
	}
	record := func(name string, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.bodies[name] = string(b)
		s.headers[name] = r.Header.Clone()
		s.mu.Unlock()
	}

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]any{
			"status": 200, "message": "OK", "notes": []string{"Follow the links."}, "data": []any{map[string]any{"name": "Fake"}},
			"_links": []any{
				map[string]any{"rel": "self", "href": "/", "method": "GET"},
				map[string]any{"rel": "dead", "href": "/gone", "method": "GET"},
				map[string]any{"rel": "mystery", "href": "/mystery", "method": "GET"},
				map[string]any{"rel": "secret", "href": "/secret", "method": "GET", "requires": []string{"agent-session"}},
			},
			"_actions": []any{
				map[string]any{"rel": "search-listings", "action": "/listings", "method": "GET", "fields": []any{
					map[string]any{"name": "make", "required": false, "schema": map[string]any{"type": "string"}},
					map[string]any{"name": "maxPrice", "required": false, "schema": map[string]any{"type": "integer", "minimum": 1}},
					map[string]any{"name": "runs", "required": false, "schema": map[string]any{"enum": []any{"drives", "does-not-run"}}},
					map[string]any{"name": "skip", "required": false, "schema": map[string]any{"type": "integer", "minimum": 0}},
					map[string]any{"name": "take", "required": false, "schema": map[string]any{"type": "integer", "minimum": 1}},
				}},
				map[string]any{"rel": "create-listing", "action": "/listings", "method": "POST", "enctype": "application/json", "fields": []any{
					map[string]any{"name": "reg", "required": true, "schema": map[string]any{"type": "string", "description": "The registration mark"}},
					map[string]any{"name": "location.lat", "required": false, "schema": map[string]any{"type": "number"}},
					map[string]any{"name": "location.lon", "required": false, "schema": map[string]any{"type": "number"}},
				}},
				map[string]any{"rel": "change-price", "action": "/listings/lst_1", "method": "PATCH", "enctype": "application/json", "ifMatch": "v3", "fields": []any{
					map[string]any{"name": "pricePence", "required": true, "schema": map[string]any{"type": "integer"}},
				}},
				map[string]any{"rel": "sign-secret", "action": "/secret", "method": "POST", "requires": []string{"agent-session"}},
			},
		})
	})

	mux.HandleFunc("GET /listings", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.queries = append(s.queries, r.URL.RawQuery)
		s.mu.Unlock()
		skip, take := 0, 2
		if v := r.URL.Query().Get("skip"); v != "" {
			skip = atoi(v)
		}
		if v := r.URL.Query().Get("take"); v != "" {
			take = atoi(v)
		}
		items := []any{}
		for i := skip; i < skip+take && i < 5; i++ {
			id := "lst_" + string(rune('0'+i))
			items = append(items, map[string]any{
				"id":     id,
				"_links": []any{map[string]any{"rel": "self", "href": "/listings/" + id, "method": "GET"}},
				"_actions": []any{map[string]any{"rel": "shortlist", "action": "/shortlist", "method": "POST", "enctype": "application/json", "fields": []any{
					map[string]any{"name": "listingId", "required": true, "value": id},
					map[string]any{"name": "note", "required": false, "schema": map[string]any{"type": "string", "maxLength": 10}},
				}}},
			})
		}
		links := []any{}
		if skip+take < 5 {
			links = append(links, map[string]any{"rel": "next", "href": "/listings?skip=" + itoa(skip+take) + "&take=" + itoa(take), "method": "GET"})
		}
		total := 5
		write(w, 200, map[string]any{"status": 200, "message": "OK", "data": items, "paging": map[string]any{"skip": skip, "take": take, "count": len(items), "total": total}, "_links": links})
	})

	mux.HandleFunc("POST /listings", func(w http.ResponseWriter, r *http.Request) {
		record("create-listing", r)
		var body map[string]any
		json.Unmarshal([]byte(s.bodies["create-listing"]), &body)
		if body["reg"] == "" {
			write(w, 422, map[string]any{"status": 422, "message": "Unable to create listing. The reg is required.", "data": []any{map[string]any{"field": "reg", "code": "required", "message": "is required"}}})
			return
		}
		write(w, 201, map[string]any{"status": 201, "message": "Created", "data": []any{map[string]any{"id": "lst_new"}}})
	})
	mux.HandleFunc("POST /shortlist", func(w http.ResponseWriter, r *http.Request) {
		record("shortlist", r)
		write(w, 201, map[string]any{"status": 201, "message": "Created", "data": []any{map[string]any{"ok": true}}})
	})
	mux.HandleFunc("PATCH /listings/lst_1", func(w http.ResponseWriter, r *http.Request) {
		record("change-price", r)
		write(w, 200, map[string]any{"status": 200, "message": "OK", "data": []any{map[string]any{"id": "lst_1"}}})
	})
	mux.HandleFunc("GET /gone", func(w http.ResponseWriter, r *http.Request) {
		write(w, 404, map[string]any{"status": 404, "message": "Unable to find it.", "data": []any{}})
	})
	mux.HandleFunc("GET /mystery", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]any{"status": 200, "message": "OK", "data": []any{}, "_links": []any{map[string]any{"rel": "self", "href": "/mystery", "method": "GET"}}})
	})
	mux.HandleFunc("GET /secret", func(w http.ResponseWriter, r *http.Request) {
		record("secret", r)
		if r.Header.Get("Authorization") != "Bearer tok" {
			write(w, 401, map[string]any{"status": 401, "message": "Sign in.", "data": []any{}})
			return
		}
		write(w, 200, map[string]any{"status": 200, "message": "OK", "data": []any{map[string]any{"ok": true}}})
	})
	mux.HandleFunc("GET /html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, "<html></html>")
	})

	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func surfer(t *testing.T, s *server, opts ...surf.Option) *surf.Surfer {
	return surftest.New(t, s.URL, append([]surf.Option{surf.WithDialects(agenttrader.New())}, opts...)...)
}

// Feature: Driving an API by its hypertext alone
//
//	Scenario: search by rel, pick an item, shortlist it, with no URL in the journey
//	  Given a server that offers a search form and gives each result its own shortlist form
//	  When the journey follows search-listings, selects item 1 and submits shortlist
//	  Then the server received a GET with the query and a POST with the item's own listing id
func TestJourney_SearchThenShortlistKnowsNoURLs(t *testing.T) {
	srv := newServer(t)
	s := surfer(t, srv)

	report, err := surf.NewWave(s).
		FromRoot().
		Then("search-listings", surf.With(surf.Values{"make": "MINI", "maxPrice": 3500})).
		ThenItem(1).
		ThenSubmit("shortlist", surf.Values{"note": "nice"}).
		Go(context.Background())
	if err != nil {
		t.Fatalf("the journey should run: %v", err)
	}

	if len(report.Lines) != 4 || report.Lines[3].Status != 201 {
		t.Errorf("report = %+v, want 4 steps ending in a 201", report.Lines)
	}
	if len(srv.queries) != 1 || srv.queries[0] != "make=MINI&maxPrice=3500" {
		t.Errorf("search query = %v, want make=MINI&maxPrice=3500 from the GET form's fields", srv.queries)
	}
	var sent map[string]any
	_ = json.Unmarshal([]byte(srv.bodies["shortlist"]), &sent)
	if sent["listingId"] != "lst_1" || sent["note"] != "nice" {
		t.Errorf("shortlist body = %v, want item 1's own id (a prefilled value) and the note", sent)
	}
}

// Scenario: a page of results is followed by its next link, never by arithmetic
func TestJourney_FollowsNextLinksToTheEnd(t *testing.T) {
	srv := newServer(t)
	page := surftest.Follow(t, surftest.Root(t, surfer(t, srv)), "search-listings")

	pages := 1
	for page.Has("next") {
		page = surftest.Follow(t, page, "next")
		pages++
	}

	if pages != 3 {
		t.Errorf("pages = %d, want 3 (5 results, 2 at a time)", pages)
	}
	if page.Resource.Paging == nil || page.Resource.Paging.Skip != 4 {
		t.Errorf("last page paging = %+v, want it to start at skip 4", page.Resource.Paging)
	}
}

// Feature: Helpful errors
//
//	Scenario: a rel the page does not offer says what it did offer
//	Scenario: a missing required input is named, with what it is for
//	Scenario: an input the form does not have is named, with the ones it does
//	Scenario: a value the schema forbids is refused before anything is sent
func TestErrors_AreHelpfulAndNothingIsSent(t *testing.T) {
	srv := newServer(t)
	home := surftest.Root(t, surfer(t, srv))

	t.Run("a rel the page does not offer says what it did offer", func(t *testing.T) {
		_, err := home.Follow(context.Background(), "new-listing")
		var nf *hypertext.RelNotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("err = %v, want a RelNotFoundError", err)
		}
		if !strings.Contains(err.Error(), "search-listings") || !strings.Contains(err.Error(), `"new-listing"`) {
			t.Errorf("error should name the missing rel and list what is offered: %v", err)
		}
	})

	t.Run("a missing required input is named, with what it is for", func(t *testing.T) {
		_, err := home.Submit(context.Background(), "create-listing", surf.Values{})
		var mi *hypertext.MissingInputError
		if !errors.As(err, &mi) || !strings.Contains(err.Error(), "reg (The registration mark)") {
			t.Errorf("err = %v, want a MissingInputError naming reg and its description", err)
		}
	})

	t.Run("an input the form does not have lists the ones it does", func(t *testing.T) {
		_, err := home.Submit(context.Background(), "create-listing", surf.Values{"reg": "AB07CDE", "colour": "red"})
		var ui *hypertext.UnexpectedInputError
		if !errors.As(err, &ui) || !strings.Contains(err.Error(), "location.lat, location.lon, reg") {
			t.Errorf("err = %v, want an UnexpectedInputError listing the inputs", err)
		}
	})

	for name, tc := range map[string]struct {
		rel    string
		values surf.Values
		reason string
	}{
		"below a minimum":     {"search-listings", surf.Values{"maxPrice": 0}, "must be at least 1"},
		"not a whole number":  {"search-listings", surf.Values{"maxPrice": 3.5}, "must be a whole number"},
		"not in the enum":     {"search-listings", surf.Values{"runs": "sort-of"}, "must be one of drives, does-not-run"},
		"wrong type":          {"search-listings", surf.Values{"make": 7}, "must be text"},
		"too long for a note": {"search-listings", surf.Values{"skip": -1}, "must be at least 0"},
	} {
		t.Run("a value the schema forbids: "+name, func(t *testing.T) {
			before := len(srv.queries)

			_, err := home.Submit(context.Background(), tc.rel, tc.values)

			var inv *hypertext.InvalidInputError
			if !errors.As(err, &inv) || !strings.Contains(err.Error(), tc.reason) {
				t.Errorf("err = %v, want an InvalidInputError saying %q", err, tc.reason)
			}
			if len(srv.queries) != before {
				t.Error("an invalid value must be refused before any request is sent")
			}
		})
	}
}

// Scenario: a refusal is read, not hidden
//
//	Given a form the server refuses with a 422
//	When the journey does not expect it, the error carries the problems
//	And when the journey expects 422, it gets the problems to assert on
func TestRefusals_AreReadAndCanBeExpected(t *testing.T) {
	srv := newServer(t)
	home := surftest.Root(t, surfer(t, srv))

	_, err := home.Submit(context.Background(), "create-listing", surf.Values{"reg": ""})
	var us *hypertext.UnexpectedStatusError
	if !errors.As(err, &us) || us.Status != 422 || !strings.Contains(err.Error(), "reg is required") {
		t.Fatalf("err = %v, want an UnexpectedStatusError with the problem in it", err)
	}

	problems := surftest.Refused(t, home, "create-listing", surf.Values{"reg": ""}, 422)
	if len(problems) != 1 || problems[0].Field != "reg" || problems[0].Code != "required" {
		t.Errorf("problems = %+v, want reg/required", problems)
	}
}

// Feature: What the control says is what is sent
//
//	Scenario: dotted names become nested JSON
//	Scenario: ifMatch becomes an If-Match header
//	Scenario: a bearer token is sent
//	Scenario: Accept names the dialects
func TestRequests_FollowTheControl(t *testing.T) {
	srv := newServer(t)
	s := surfer(t, srv, surf.WithBearer("tok"))
	home := surftest.Root(t, s)

	surftest.Submit(t, home, "create-listing", surf.Values{"reg": "AB07CDE", "location.lat": 51.06, "location.lon": 0.16})
	var created map[string]any
	_ = json.Unmarshal([]byte(srv.bodies["create-listing"]), &created)
	loc, _ := created["location"].(map[string]any)
	if loc["lat"] != 51.06 || loc["lon"] != 0.16 {
		t.Errorf("body = %v, want location nested as an object", created)
	}
	if ct := srv.headers["create-listing"].Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want the control's enctype", ct)
	}

	surftest.Submit(t, home, "change-price", surf.Values{"pricePence": 350000})
	if got := srv.headers["change-price"].Get("If-Match"); got != "v3" {
		t.Errorf("If-Match = %q, want v3 from the control's ifMatch", got)
	}

	if got := srv.headers["create-listing"].Get("Authorization"); got != "Bearer tok" {
		t.Errorf("Authorization = %q, want the bearer token", got)
	}
	if got := srv.headers["create-listing"].Get("Accept"); !strings.Contains(got, "application/json") {
		t.Errorf("Accept = %q, want the dialects' media types", got)
	}
}

// Scenario: a response no dialect can read says which dialects were tried
func TestUnreadableResponses_NameTheDialects(t *testing.T) {
	srv := newServer(t)
	s := surftest.New(t, srv.URL+"/html", surf.WithDialects(agenttrader.New()))

	_, err := s.Root(context.Background())

	var um *hypertext.UnsupportedMediaTypeError
	if !errors.As(err, &um) || !strings.Contains(err.Error(), "agenttrader") || !strings.Contains(err.Error(), "text/html") {
		t.Errorf("err = %v, want an UnsupportedMediaTypeError naming text/html and the dialects tried", err)
	}
}

// Feature: Exploring
//
//	Scenario: reachability, dead links, unknown rels and what was skipped
func TestExplore_ReportsWhatIsReachableAndWhatIsWrong(t *testing.T) {
	srv := newServer(t)
	s := surfer(t, srv)
	known := map[string]bool{"self": true, "dead": true, "secret": true, "next": true, "search-listings": true, "create-listing": true, "change-price": true, "sign-secret": true, "shortlist": true}

	report, err := s.Explore(context.Background(), surf.ExploreOptions{KnownRel: func(r string) bool { return known[r] }})
	if err != nil {
		t.Fatal(err)
	}

	has := func(kind, contains string) bool {
		for _, f := range report.Findings {
			if f.Kind == kind && strings.Contains(f.Message, contains) {
				return true
			}
		}
		return false
	}
	if !has("dead-link", "dead") {
		t.Errorf("a link that answers 404 should be reported as a dead link: %+v", report.Findings)
	}
	if !has("unknown-rel", `"mystery"`) {
		t.Errorf("a rel not in the registry should be reported: %+v", report.Findings)
	}

	visited := map[string]bool{}
	for _, v := range report.Visits {
		visited[v.URL] = true
	}
	for _, want := range []string{"/", "/listings", "/mystery"} {
		if !visited[want] {
			t.Errorf("%s should have been reached by following links. Visited: %v", want, visited)
		}
	}
	if visited["/secret"] {
		t.Error("a control that needs a session must not be followed without a token")
	}

	skipped := strings.Join(report.Skipped, "\n")
	for _, want := range []string{"agent-session", "changes state"} {
		if !strings.Contains(skipped, want) {
			t.Errorf("the report should say why controls were skipped (%q): %s", want, skipped)
		}
	}
}
