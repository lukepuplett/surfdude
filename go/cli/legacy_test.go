package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Feature: show and explore, the first two commands, work as they always did

// linksAPI is an API whose front door has the links it is given, and where every other path is a 404.
func linksAPI(t *testing.T, links string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Add("Link", profileLink)
		_, _ = io.WriteString(w, `{"status":200,"message":"OK","data":[],"_links":[`+links+`]}`)
	}))
	t.Cleanup(srv.Close)

	return srv
}

// Scenario: show prints the front door: its address, status, dialect, notes and every control with its fields
func TestShow_PrintsTheFrontDoor(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, "show", api.URL)

	if r.Code != ExitOK {
		t.Fatalf("exit %d\nstderr: %s", r.Code, r.ErrOut)
	}
	for _, want := range []string{
		api.URL + "  200  read by the agenttrader dialect",
		"  note: This is a fake.",
		"form   POST   register",
		"- email (required)",
		"link   GET    health",
	} {
		if !strings.Contains(r.Out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.Out)
		}
	}
}

// Scenario: a root alone does the same as show
func TestRootAloneShowsTheFrontDoor(t *testing.T) {
	api := newFakeAPI(t)

	alone := runCLI(t, "", nil, api.URL)
	shown := runCLI(t, "", nil, "show", api.URL)

	if alone.Code != ExitOK || alone.Out != shown.Out {
		t.Fatalf("a root alone printed something other than show:\n%s\nversus\n%s", alone.Out, shown.Out)
	}
}

// Scenario: show exits 1 for any failure, as it always did
func TestShow_FailureExits1(t *testing.T) {
	if r := runCLI(t, "", nil, "show", "http://127.0.0.1:1"); r.Code != 1 || r.ErrOut == "" {
		t.Fatalf("exit %d, stderr %q, want 1 and a message", r.Code, r.ErrOut)
	}
	if r := runCLI(t, "", nil, "-dialects", "nope", "show", "http://127.0.0.1:1"); r.Code != 1 {
		t.Fatalf("an unknown dialect exit %d, want 1 as before", r.Code)
	}
	if r := runCLI(t, "", nil, "show"); r.Code != ExitUsage {
		t.Fatalf("show with no address exit %d, want 2 as before", r.Code)
	}
}

// Scenario: explore crawls, and exits 0 when all is well
func TestExplore_CleanExits0(t *testing.T) {
	srv := linksAPI(t, `{"rel":"self","href":"/","method":"GET"}`)

	r := runCLI(t, "", nil, "explore", srv.URL)

	if r.Code != ExitOK || !strings.Contains(r.Out, "page(s) from the front door by following links:") || !strings.Contains(r.Out, "No problems found.") {
		t.Fatalf("exit %d, stdout:\n%s", r.Code, r.Out)
	}
}

// Scenario: explore exits 1 when it finds a problem, and lists it
func TestExplore_ProblemExits1(t *testing.T) {
	srv := linksAPI(t, `{"rel":"self","href":"/","method":"GET"},{"rel":"gone","href":"/gone","method":"GET"}`)

	r := runCLI(t, "", nil, "explore", srv.URL)

	if r.Code != 1 || !strings.Contains(r.Out, "Problems (") {
		t.Fatalf("exit %d, stdout:\n%s", r.Code, r.Out)
	}
}

// Scenario: explore reports a rel that is not in the registry it is given
func TestExplore_KnownRelsRegistry(t *testing.T) {
	srv := linksAPI(t, `{"rel":"self","href":"/","method":"GET"},{"rel":"mystery-rel","href":"/","method":"GET"}`)
	registry := filepath.Join(t.TempDir(), "rels.md")
	if err := os.WriteFile(registry, []byte("| `self` | link | here | none |\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	r := runCLI(t, "", nil, "-known-rels", registry, "explore", srv.URL)

	if r.Code != 1 || !strings.Contains(r.Out, "mystery-rel") {
		t.Fatalf("exit %d, want 1 naming the unregistered rel\n%s", r.Code, r.Out)
	}
	if r := runCLI(t, "", nil, "-known-rels", filepath.Join(t.TempDir(), "missing.md"), "explore", srv.URL); r.Code != 1 {
		t.Fatalf("a missing registry file exit %d, want 1", r.Code)
	}
}
