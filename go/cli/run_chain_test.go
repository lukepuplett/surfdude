package cli

import (
	"strings"
	"testing"
)

// Feature: surf ROOT STEP then STEP then STEP (a chain)

const sandboxChain = "register email=me@example.test " +
	"then outbox to=me@example.test --keep code=data.0.text~[0-9]{6} " +
	"then verify-email email=me@example.test code={code} --keep cred=data.0.credential " +
	"then sign-in credential={cred}"

// Scenario: a chain registers, reads the emailed code, verifies it and signs in
func TestRun_ChainRunsEveryStepAndPassesValuesForward(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, append([]string{"-q", "data.0.token", api.URL}, strings.Fields(sandboxChain)...)...)

	if r.Code != ExitOK || r.Out != "tok_signed\n" {
		t.Fatalf("exit %d, stdout %q, want 0 and tok_signed\nstderr: %s", r.Code, r.Out, r.ErrOut)
	}
	var steps []string
	for _, q := range api.all() {
		steps = append(steps, q.Method+" "+q.Path)
	}
	want := "GET / | POST /auth/email/start | GET /sandbox/outbox | POST /auth/email/verify | POST /auth/token"
	if got := strings.Join(steps, " | "); got != want {
		t.Fatalf("requests = %s\nwant       %s", got, want)
	}
	for _, q := range api.all() {
		switch q.Path {
		case "/auth/email/verify":
			if q.Body != `{"code":"654321","email":"me@example.test"}` {
				t.Errorf("verify body = %s, want the code kept from the mailbox", q.Body)
			}
		case "/auth/token":
			if q.Body != `{"credential":"agt_testcredential"}` {
				t.Errorf("sign-in body = %s, want the credential kept from verify", q.Body)
			}
		}
	}
}

// Scenario: only the last step's response is printed, and the earlier steps announce themselves
func TestRun_ChainPrintsOnlyTheLastResponse(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, append([]string{api.URL}, strings.Fields(sandboxChain)...)...)

	if strings.Contains(r.Out, "202 Accepted") || strings.Contains(r.Out, "Your code is") {
		t.Fatalf("stdout shows an earlier step's response:\n%s", r.Out)
	}
	for _, want := range []string{"POST register -> ", "POST verify-email -> ", "POST sign-in -> "} {
		if !strings.Contains(r.ErrOut, want) {
			t.Errorf("stderr lacks %q:\n%s", want, r.ErrOut)
		}
	}
}

// The tests below are about what a chain does, not about how it finds a step, and they chain to rels
// that only the front door offers, so they pass --front-door. How a step is found is in
// frontdoor_test.go.

// Scenario: the chain stops at the first failing step and exits with that step's code
func TestRun_ChainStopsAtTheFirstFailure(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, "--front-door", api.URL, "register", "email=a@b.test", "then", "refuse", "then", "boom")

	if r.Code != ExitRefused {
		t.Fatalf("exit %d, want 1, the code of the refused step\nstderr: %s", r.Code, r.ErrOut)
	}
	if api.count("POST", "/boom") != 0 {
		t.Fatal("a step after the failure was sent")
	}
}

// Scenario: an unknown {name} is refused before anything at all is sent, not even the front door
func TestRun_UnknownTemplateSendsNothing(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, api.URL, "register", "email={nobody}")

	if r.Code != ExitUsage || !strings.Contains(r.ErrOut, "{nobody}") {
		t.Fatalf("exit %d, stderr %q, want 2 and the name", r.Code, r.ErrOut)
	}
	if n := len(api.all()); n != 0 {
		t.Fatalf("%d request(s) were sent, want none", n)
	}
}

// Scenario: a --keep path that finds nothing stops the chain
func TestRun_KeepWithAMissingPathStopsTheChain(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, api.URL, "register", "email=a@b.test", "--keep", "c=data.0.nope", "then", "set-count", "count:=1")

	if r.Code != ExitUsage || !strings.Contains(r.ErrOut, "data.0.nope") {
		t.Fatalf("exit %d, stderr %q, want 2 naming the path", r.Code, r.ErrOut)
	}
	if api.count("POST", "/count") != 0 {
		t.Fatal("a step after the failed --keep was sent")
	}
}

// Scenario: --keep can pull a pattern out of text, such as the six digits in an email
func TestRun_KeepWithAPatternTakesTheMatch(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, "--front-door", "-q", "data.0.stored", api.URL, "outbox", "to=a@b.test", "--keep", "code=data.0.text~[0-9]{6}", "then", "set-count", "count:=1", "note={code}")

	if r.Code != ExitOK || !strings.Contains(api.last().Body, `"note":"654321"`) {
		t.Fatalf("exit %d, body %q, want the note to be exactly the match 654321\nstderr: %s", r.Code, api.last().Body, r.ErrOut)
	}

	// With a group, the group is what is kept.
	api2 := newFakeAPI(t)
	runCLI(t, "", nil, "--front-door", api2.URL, "outbox", "to=a@b.test", "--keep", `code=data.0.text~is ([0-9]+)`, "then", "set-count", "count:=1", "note={code}")
	if !strings.Contains(api2.last().Body, `"note":"654321"`) {
		t.Fatalf("body %q, want the first group to be kept", api2.last().Body)
	}

	// A pattern that does not match is an error, not an empty value.
	api3 := newFakeAPI(t)
	if r := runCLI(t, "", nil, "--front-door", api3.URL, "outbox", "to=a@b.test", "--keep", `code=data.0.text~zzz`, "then", "set-count", "count:=1"); r.Code != ExitUsage {
		t.Fatalf("a pattern that does not match exit %d, want 2", r.Code)
	}
	if api3.count("POST", "/count") != 0 {
		t.Fatal("a step after a pattern that did not match was sent")
	}
}

// Scenario: a value kept from a secret field is never printed, in the announcement, the summary or an error
func TestRun_KeptSecretsAreNeverPrinted(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, append([]string{api.URL}, strings.Fields(sandboxChain)...)...)

	for _, secret := range []string{"agt_testcredential", "tok_signed", "tok_verify"} {
		if strings.Contains(r.Out, secret) || strings.Contains(r.ErrOut, secret) {
			t.Errorf("%s was printed:\nstdout: %s\nstderr: %s", secret, r.Out, r.ErrOut)
		}
	}
	if !strings.Contains(r.Out, "token=<secret>") {
		t.Errorf("the summary does not show that a token was hidden:\n%s", r.Out)
	}
}
