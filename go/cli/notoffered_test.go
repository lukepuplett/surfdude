package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lukepuplett/surfdude/go/hypertext"
)

// Feature: exit 5, a rel that is not offered
//
// Luke's reasoning: a typo and a missing control cannot be told apart, like command not found in bash,
// which has its own exit code (127). So one code says "the name you gave is not offered by the page it
// was looked for on", and exit 2 stays for a command line that is badly formed.

// Scenario: every place a rel is looked up exits 5 when it is not there
func TestNotOffered_EveryLookupSiteExits5(t *testing.T) {
	env := map[string]string{"CRED": "agt_testcredential"}
	cases := []struct {
		name string
		args []string
	}{
		{"the first step, which reads the front door", []string{"nope"}},
		{"a later step, offered by nothing", []string{"bare", "then", "nope"}},
		{"a later step, offered only by the front door and not asked for", []string{"bare", "then", "set-count", "count:=1"}},
		{"describe", []string{"__describe__", "nope"}},
		{"the sign-in rel, when the 401 page does not offer it and --front-door was not given", []string{"__signin__", "locked"}},
	}

	for _, c := range cases {
		api := newFakeAPI(t)
		var args []string
		switch c.args[0] {
		case "__describe__":
			args = []string{"describe", api.URL, c.args[1]}
		case "__signin__":
			args = []string{"--sign-in", "credential=env:CRED", api.URL, c.args[1]}
		default:
			args = append([]string{api.URL}, c.args...)
		}

		r := runCLI(t, "", env, args...)

		if r.Code != ExitNotOffered {
			t.Errorf("%s: exit %d, want %d\nstderr: %s", c.name, r.Code, ExitNotOffered, r.ErrOut)
		}
		if post := len(api.all()) - api.count("GET", "/") - api.count("GET", "/bare") - api.count("GET", "/locked"); post != 0 {
			t.Errorf("%s: %d request(s) beyond the reads were sent, want none", c.name, post)
		}
	}
}

// Scenario: the sign-in rel missing from the server altogether is exit 5 as well
func TestNotOffered_NoSignInAnywhereExits5(t *testing.T) {
	srv := bareAPI(t)

	r := runCLI(t, "", map[string]string{"CRED": "x"}, "--sign-in", "credential=env:CRED", srv.URL, "protected")

	if r.Code != ExitNotOffered || !strings.Contains(r.ErrOut, "sign-in") {
		t.Fatalf("exit %d, stderr %q, want 5 naming the missing sign-in", r.Code, r.ErrOut)
	}
}

// Scenario: a script line that names a rel the page does not offer is exit 5
func TestNotOffered_AScriptExits5(t *testing.T) {
	api := newFakeAPI(t)
	path := writeScript(t, "root "+api.URL+"\nnope\n")

	if r := runCLI(t, "", nil, "run", path); r.Code != ExitNotOffered {
		t.Fatalf("exit %d, want 5\nstderr: %s", r.Code, r.ErrOut)
	}
}

// Scenario: the library's own rel-not-found error is exit 5 too, even when it is wrapped
func TestNotOffered_TheLibrarysErrorMapsToo(t *testing.T) {
	err := fmt.Errorf("step failed: %w", &hypertext.RelNotFoundError{Rel: "x", Where: "The page", Offered: []string{"y"}})

	if got := exitFor(err); got != ExitNotOffered {
		t.Fatalf("exitFor = %d, want %d", got, ExitNotOffered)
	}
}

// Scenario: a badly formed command is still exit 2, and nothing about it is "not offered"
func TestNotOffered_UsageErrorsAreStillExit2(t *testing.T) {
	cases := map[string][]string{
		"a missing required field":        {"register"},
		"a field the form does not have":  {"register", "email=a@b.test", "colour=red"},
		"text where a number belongs":     {"set-count", "count=abc"},
		"an argument that is not a value": {"register", "email"},
	}
	for name, args := range cases {
		api := newFakeAPI(t)

		r := runCLI(t, "", nil, append([]string{api.URL}, args...)...)

		if r.Code != ExitUsage {
			t.Errorf("%s: exit %d, want 2\nstderr: %s", name, r.Code, r.ErrOut)
		}
	}
	if r := runCLI(t, "", nil, "--no-such-flag", "http://x.test", "register"); r.Code != ExitUsage {
		t.Errorf("an unknown flag: exit %d, want 2", r.Code)
	}
	if got := exitFor(&usageError{msg: "x"}); got != ExitUsage {
		t.Errorf("a usage error maps to %d, want 2", got)
	}
}

// Scenario: a typo that is close to exactly one offered rel gets a "Did you mean"
func TestNotOffered_ATypoGetsASuggestion(t *testing.T) {
	cases := map[string]string{
		"registr":  "register",  // one letter short
		"regster":  "register",  // one letter missing from the middle
		"sign":     "sign-in",   // contained in exactly one rel
		"set-cont": "set-count", // close, and not close to the other set-* rels
	}
	for typo, want := range cases {
		api := newFakeAPI(t)

		r := runCLI(t, "", nil, api.URL, typo)

		if r.Code != ExitNotOffered || !strings.Contains(r.ErrOut, `Did you mean "`+want+`"?`) {
			t.Errorf("%q: exit %d, stderr %q, want exit 5 and a suggestion of %q", typo, r.Code, r.ErrOut, want)
		}
	}
}

// Scenario: no suggestion when nothing is close, and none when several are
func TestNotOffered_NoSuggestionWhenNoneOrSeveralAreClose(t *testing.T) {
	for name, typo := range map[string]string{"nothing is close": "zzzzzz", "several contain it": "set"} {
		api := newFakeAPI(t)

		r := runCLI(t, "", nil, api.URL, typo)

		if r.Code != ExitNotOffered {
			t.Errorf("%s: exit %d, want 5", name, r.Code)
		}
		if strings.Contains(r.ErrOut, "Did you mean") {
			t.Errorf("%s: a suggestion was made:\n%s", name, r.ErrOut)
		}
		if !strings.Contains(r.ErrOut, "register") {
			t.Errorf("%s: the message does not list what the front door offers:\n%s", name, r.ErrOut)
		}
	}
}

func TestSuggest(t *testing.T) {
	offered := []string{"register", "verify-email", "sign-in", "set-count", "set-password"}
	cases := []struct {
		rel  string
		want string
		ok   bool
	}{
		{"registr", "register", true},
		{"REGISTER", "", false}, // an exact match (any case) is not a typo, so it is not suggested
		{"signin", "sign-in", true},
		{"sign-in-now", "sign-in", true}, // the offered rel is contained in what was typed
		{"verfy-email", "verify-email", true},
		{"set", "", false},         // contained in several
		{"zzz", "", false},         // close to nothing
		{"", "", false},            // an empty name asks for nothing
		{"reg-is-ter!", "", false}, // three edits away
	}

	for _, c := range cases {
		got, ok := suggest(c.rel, offered)
		if got != c.want || ok != c.ok {
			t.Errorf("suggest(%q) = %q, %v, want %q, %v", c.rel, got, ok, c.want, c.ok)
		}
	}
	// A rel listed twice is one candidate, not two.
	if got, ok := suggest("registr", []string{"register", "register"}); !ok || got != "register" {
		t.Errorf("a rel listed twice gave %q, %v, want register", got, ok)
	}
}

// Scenario: the usage text documents exit 5
func TestNotOffered_UsageTextDocumentsExit5(t *testing.T) {
	r := runCLI(t, "", nil, "help")

	if !strings.Contains(r.Out, "5 rel not offered") {
		t.Fatalf("the usage text does not document exit 5:\n%s", r.Out)
	}
}
