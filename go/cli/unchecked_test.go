package cli

import (
	"strings"
	"testing"
)

// Feature: --unchecked, to send what the form forbids
//
// The form is honoured by default: an unknown field, a missing required one or a value outside its
// schema is refused before anything is sent. A probe that wants to see whether the SERVER refuses
// those too says --unchecked on that step, and every such step says so on stderr.

const uncheckedNote = "note: sending unchecked: these values are not checked against the form."

// Scenario: a value the form forbids is refused locally, and reaches the server only with the flag
func TestUncheckedFlag_AValueTheFormForbidsReachesTheServerOnlyWithTheFlag(t *testing.T) {
	api := newFakeAPI(t)

	checked := runCLI(t, "", nil, api.URL, "set-count", "count:=99") // the form says at most 10
	if checked.Code != ExitUsage || api.count("POST", "/count") != 0 {
		t.Fatalf("checked: exit %d, %d request(s) to /count, want a usage error and none\nstderr: %s",
			checked.Code, api.count("POST", "/count"), checked.ErrOut)
	}

	r := runCLI(t, "", nil, api.URL, "set-count", "count:=99", "--unchecked")

	if r.Code != ExitOK {
		t.Fatalf("unchecked: exit %d, want 0\nstderr: %s", r.Code, r.ErrOut)
	}
	if got := api.count("POST", "/count"); got != 1 {
		t.Fatalf("%d request(s) to /count, want 1", got)
	}
	for _, q := range api.all() {
		if q.Path == "/count" && q.Body != `{"count":99}` {
			t.Errorf("body = %s, want the value exactly as given", q.Body)
		}
	}
	if !strings.Contains(r.ErrOut, uncheckedNote) {
		t.Errorf("stderr lacks the unchecked note:\n%s", r.ErrOut)
	}
}

// Scenario: an unknown field is sent, and a required field may be left out
func TestUncheckedFlag_SendsAnUnknownFieldAndLeavesARequiredOneOut(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, api.URL, "set-count", "bogus=1", "--unchecked") // count is required, bogus is not a field

	if r.Code != ExitOK {
		t.Fatalf("exit %d, want 0\nstderr: %s", r.Code, r.ErrOut)
	}
	for _, q := range api.all() {
		if q.Path == "/count" && q.Body != `{"bogus":"1"}` {
			t.Errorf("body = %s, want {\"bogus\":\"1\"}: the unknown field, and no count", q.Body)
		}
	}
}

// Scenario: a dry run of an unchecked step shows the request, sends nothing and still hides secrets
func TestUncheckedFlag_DryRunShowsTheRequestAndStillMasksSecrets(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, "--dry-run", api.URL, "set-password", "password=hunter2", "email=a@example.test", "bogus=1", "--unchecked")

	if r.Code != ExitOK {
		t.Fatalf("exit %d, want 0\nstderr: %s", r.Code, r.ErrOut)
	}
	if !strings.Contains(r.Out, `"bogus":"1"`) {
		t.Errorf("the plan should show the unchecked field:\n%s", r.Out)
	}
	if strings.Contains(r.Out+r.ErrOut, "hunter2") {
		t.Fatalf("the secret was printed:\nstdout: %s\nstderr: %s", r.Out, r.ErrOut)
	}
	if !strings.Contains(r.Out, secretMask) {
		t.Errorf("the secret should show as %s:\n%s", secretMask, r.Out)
	}
	if !strings.Contains(r.ErrOut, uncheckedNote) {
		t.Errorf("a dry run of an unchecked step should say so too:\n%s", r.ErrOut)
	}
	if got := api.count("POST", "/password"); got != 0 {
		t.Fatalf("a dry run sent %d request(s) to /password", got)
	}
}

// Scenario: the flag belongs to its step: a later step in the same chain is still checked
func TestUncheckedFlag_AppliesToThatStepOnly(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, "--front-door", api.URL,
		"set-count", "count:=99", "--unchecked",
		"then", "set-count", "count:=98")

	if r.Code != ExitUsage {
		t.Fatalf("exit %d, want %d: the second step is checked and 98 is more than 10\nstderr: %s", r.Code, ExitUsage, r.ErrOut)
	}
	if got := api.count("POST", "/count"); got != 1 {
		t.Fatalf("%d request(s) to /count, want only the first, unchecked step", got)
	}
}

// Scenario: the note never repeats a value
func TestUncheckedFlag_TheNoteNamesNoValue(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, api.URL, "set-count", "count:=99", "--unchecked")

	for _, line := range strings.Split(r.ErrOut, "\n") {
		if strings.HasPrefix(line, "note:") && strings.Contains(line, "99") {
			t.Fatalf("a note repeats a value: %q", line)
		}
	}
}

// Scenario: --unchecked given twice on one step is a usage error, and nothing is sent
func TestUncheckedFlag_GivenTwiceIsAUsageError(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, api.URL, "set-count", "count:=3", "--unchecked", "--unchecked")

	if r.Code != ExitUsage || len(api.all()) != 0 {
		t.Fatalf("exit %d with %d request(s), want a usage error and none", r.Code, len(api.all()))
	}
}
