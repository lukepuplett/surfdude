package cli

import (
	"strings"
	"testing"
)

// Feature: a step's rel must be offered by the page it starts from (--front-door allows the fallback)
//
// Luke's rule: a control that is not on the page you are on is the API's bug, as it would be on a
// website. Going back to the front door is allowed only when asked for, and it says so.

const fallbackNote = `note: "set-count" is not on the last response, so it was taken from the front door.`

// Scenario: by default a step must be offered by the last response, and the error says what to do
func TestFrontDoor_ByDefaultAStepMustBeOfferedByTheLastResponse(t *testing.T) {
	api := newFakeAPI(t)

	// bare offers nothing, and set-count is only on the front door.
	r := runCLI(t, "", nil, api.URL, "bare", "then", "set-count", "count:=2")

	if r.Code == ExitOK {
		t.Fatalf("exit 0: a rel that the last response does not offer was taken from the front door without being asked\nstderr: %s", r.ErrOut)
	}
	for _, want := range []string{`"set-count"`, "front door offers it", "last response does not", "--front-door"} {
		if !strings.Contains(r.ErrOut, want) {
			t.Errorf("stderr lacks %q:\n%s", want, r.ErrOut)
		}
	}
	if api.count("POST", "/count") != 0 {
		t.Fatal("the step was sent although it was not offered")
	}
}

// Scenario: with --front-door the step is taken from the front door, and one line on stderr says so
func TestFrontDoor_TheFlagAllowsTheFallbackAndSaysSo(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, "--front-door", api.URL, "bare", "then", "set-count", "count:=2")

	if r.Code != ExitOK || api.count("POST", "/count") != 1 {
		t.Fatalf("exit %d, %d POST /count, want 0 and 1\nstderr: %s", r.Code, api.count("POST", "/count"), r.ErrOut)
	}
	if got := strings.Count(r.ErrOut, fallbackNote); got != 1 {
		t.Fatalf("stderr has the fallback note %d times, want once:\n%s", got, r.ErrOut)
	}
	if strings.Contains(r.Out, "taken from the front door") {
		t.Fatalf("the note went to stdout, which a script reads:\n%s", r.Out)
	}
}

// Scenario: the flag says nothing when the last response offered the rel anyway
func TestFrontDoor_NoNoteWhenTheLastResponseOffersTheRel(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, "--front-door", api.URL, "register", "email=a@b.test", "then", "verify-email", "email=a@b.test", "code=654321")

	if r.Code != ExitOK {
		t.Fatalf("exit %d\nstderr: %s", r.Code, r.ErrOut)
	}
	if strings.Contains(r.ErrOut, "taken from the front door") {
		t.Fatalf("a note was printed although every step was offered by the page before it:\n%s", r.ErrOut)
	}
}

// Scenario: the first step reads the front door, so it never needs the flag
func TestFrontDoor_TheFirstStepNeverNeedsTheFlag(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, api.URL, "register", "email=a@b.test")

	if r.Code != ExitOK || strings.Contains(r.ErrOut, "taken from the front door") {
		t.Fatalf("exit %d, stderr %q, want 0 and no note", r.Code, r.ErrOut)
	}
}

// Scenario: the sign-in lookup follows the same rule, and the 401 page is the page it looks at
func TestFrontDoor_TheSignInLookupObeysTheSameRule(t *testing.T) {
	env := map[string]string{"CRED": "agt_testcredential"}

	// By default: locked's 401 page offers no sign-in, so there is nothing to follow.
	api := newFakeAPI(t)
	r := runCLI(t, "", env, "--sign-in", "credential=env:CRED", api.URL, "locked")
	if r.Code == ExitOK || !strings.Contains(r.ErrOut, "--front-door") {
		t.Fatalf("exit %d, stderr %q, want a failure that names --front-door", r.Code, r.ErrOut)
	}
	if api.count("POST", "/auth/token") != 0 {
		t.Fatal("signed in although the 401 page did not offer it and --front-door was not given")
	}

	// With the flag: it is taken from the front door, with a note, and the step is retried.
	api = newFakeAPI(t)
	r = runCLI(t, "", env, "--front-door", "--sign-in", "credential=env:CRED", api.URL, "locked")
	if r.Code != ExitOK || !strings.Contains(r.Out, "hello=locked-you") {
		t.Fatalf("exit %d, stdout %q\nstderr: %s", r.Code, r.Out, r.ErrOut)
	}
	if want := `note: "sign-in" is not on the last response, so it was taken from the front door.`; !strings.Contains(r.ErrOut, want) {
		t.Fatalf("stderr lacks %q:\n%s", want, r.ErrOut)
	}
}

// Scenario: a script follows the flag as it is given
func TestFrontDoor_AScriptFollowsTheFlagAsGiven(t *testing.T) {
	api := newFakeAPI(t)
	path := writeScript(t, "root "+api.URL+"\nbare\nset-count count:=1\n")

	if r := runCLI(t, "", nil, "run", path); r.Code == ExitOK {
		t.Fatalf("a script took a rel from the front door without --front-door\nstderr: %s", r.ErrOut)
	}
	if r := runCLI(t, "", nil, "--front-door", "run", path); r.Code != ExitOK {
		t.Fatalf("exit %d, want 0 with --front-door\nstderr: %s", r.Code, r.ErrOut)
	}
}
