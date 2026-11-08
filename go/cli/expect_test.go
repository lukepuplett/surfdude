package cli

import (
	"strings"
	"testing"
)

// Feature: --expect STATUS, for a step that is meant to be refused
//
// A chain stops at the first refusal, so "this is refused with a 429, and then carry on" could not be
// scripted. --expect makes the listed statuses the success of that step, and the page that answered
// is an ordinary page: it prints, its controls are what the next step may use, and --keep reads it.

// Scenario: a step refused as expected is a normal page, and the chain carries on from it
//
//	Given an address that was sent a code a moment ago
//	When a chain registers it, expects the 429, and verifies the code from the refusal page
//	Then every step succeeds, and verify-email was taken from the 429 page, not from the front door
func TestExpect_ARefusedStepIsAPageAndTheChainCarriesOn(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, api.URL, "register", "email=slow@example.test", "--expect", "429",
		"then", "verify-email", "email=slow@example.test", "code=654321")

	if r.Code != ExitOK {
		t.Fatalf("exit %d, want 0\nstderr: %s", r.Code, r.ErrOut)
	}
	if api.count("POST", "/auth/email/start") != 1 || api.count("POST", "/auth/email/verify") != 1 {
		t.Fatalf("requests = %+v, want one start (refused) and one verify", api.all())
	}
	if !strings.Contains(r.Out, "200 OK") {
		t.Errorf("stdout should be the last step's response:\n%s", r.Out)
	}
	if strings.Contains(r.ErrOut, "front door") {
		t.Errorf("verify-email should come from the 429 page, but stderr mentions the front door:\n%s", r.ErrOut)
	}
}

// Scenario: the motivating shape: register, a second register refused with 429, then verify-email from that page
func TestExpect_RegisterThenARefusedSecondRegisterThenVerify(t *testing.T) {
	api := newFakeAPI(t)

	// The second register is not offered by the first one's page, so this run allows the front door.
	r := runCLI(t, "", nil, "--front-door", api.URL,
		"register", "email=me@example.test",
		"then", "register", "email=slow@example.test", "--expect", "429",
		"then", "verify-email", "email=slow@example.test", "code=654321")

	if r.Code != ExitOK {
		t.Fatalf("exit %d, want 0\nstderr: %s", r.Code, r.ErrOut)
	}
	if got := api.count("POST", "/auth/email/start"); got != 2 {
		t.Fatalf("register was sent %d times, want 2", got)
	}
	if api.count("POST", "/auth/email/verify") != 1 {
		t.Fatalf("verify-email was not reached after the refusal: %+v", api.all())
	}
}

// Scenario: when the expected refusal is the last step, it prints on stdout and the exit is 0
func TestExpect_AnExpectedRefusalAsTheLastStepPrintsAndSucceeds(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, api.URL, "register", "email=slow@example.test", "--expect", "429")

	if r.Code != ExitOK {
		t.Fatalf("exit %d, want 0\nstderr: %s", r.Code, r.ErrOut)
	}
	for _, want := range []string{"429", "wait before asking again", "verify-email"} {
		if !strings.Contains(r.Out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.Out)
		}
	}
}

// Scenario: a status the step did not expect still fails it, with the usual exit code
func TestExpect_ASurpriseStatusStillFails(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, api.URL, "refuse", "--expect", "429") // refuse answers 422

	if r.Code != ExitRefused {
		t.Fatalf("exit %d, want %d: a 422 was not what the step expected\nstderr: %s", r.Code, ExitRefused, r.ErrOut)
	}
	if !strings.Contains(r.ErrOut, "422") {
		t.Errorf("stderr should show the 422 that surprised the step:\n%s", r.ErrOut)
	}
}

// Scenario: the listed statuses are the whole of success, so a 2xx is no longer enough
func TestExpect_ReplacesTheDefaultOfAny2xx(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, api.URL, "register", "email=me@example.test", "--expect", "200") // answers 202

	if r.Code != ExitRefused {
		t.Fatalf("exit %d, want %d: a 202 is not the 200 that was expected\nstderr: %s", r.Code, ExitRefused, r.ErrOut)
	}
}

// Scenario: several statuses can be expected, and any of them is success
func TestExpect_ACommaListExpectsSeveralStatuses(t *testing.T) {
	api := newFakeAPI(t)

	for _, list := range []string{"429,422", "422,429", "422"} {
		if r := runCLI(t, "", nil, api.URL, "refuse", "--expect", list); r.Code != ExitOK {
			t.Errorf("--expect %s: exit %d, want 0 (refuse answers 422)\nstderr: %s", list, r.Code, r.ErrOut)
		}
	}
}

// Scenario: the strict rule still applies after an expected refusal: the next rel must be on that page
func TestExpect_TheNextStepMustBeOfferedByTheRefusalPage(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, api.URL, "register", "email=slow@example.test", "--expect", "429",
		"then", "sign-in", "credential=x") // the 429 page offers verify-email, not sign-in

	if r.Code != ExitNotOffered {
		t.Fatalf("exit %d, want %d (not offered)\nstderr: %s", r.Code, ExitNotOffered, r.ErrOut)
	}
	if !strings.Contains(r.ErrOut, "verify-email") {
		t.Errorf("the error should list what the 429 page does offer:\n%s", r.ErrOut)
	}
}

// Scenario: a step that expects a 401 is testing the refusal, so it does not sign in
func TestExpect_A401IsNotAnInvitationToSignIn(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", map[string]string{"CRED": "agt_testcredential"}, "--sign-in", "credential=env:CRED",
		api.URL, "protected", "--expect", "401")

	if r.Code != ExitOK {
		t.Fatalf("exit %d, want 0\nstderr: %s", r.Code, r.ErrOut)
	}
	if api.count("POST", "/auth/token") != 0 {
		t.Fatal("a step that expected the 401 still signed in")
	}
}

// Scenario: a status outside 100 to 599, a list that is not numbers, or a repeat, is a usage error
//
//	and nothing is sent, not even the read of the front door
func TestExpect_BadValuesAreUsageErrors(t *testing.T) {
	cases := map[string][]string{
		"no value":              {"register", "email=a@b.test", "--expect"},
		"not a number":          {"register", "email=a@b.test", "--expect", "abc"},
		"below 100":             {"register", "email=a@b.test", "--expect", "99"},
		"above 599":             {"register", "email=a@b.test", "--expect", "600"},
		"a trailing comma":      {"register", "email=a@b.test", "--expect", "200,"},
		"one bad among good":    {"register", "email=a@b.test", "--expect", "429,x"},
		"an empty value":        {"register", "email=a@b.test", "--expect", ""},
		"given twice":           {"register", "email=a@b.test", "--expect", "429", "--expect", "422"},
		"before the root as -x": {"-expect", "429"},
	}

	for name, args := range cases {
		api := newFakeAPI(t)
		var full []string
		if name == "before the root as -x" {
			full = append(args, api.URL, "register", "email=a@b.test")
		} else {
			full = append([]string{api.URL}, args...)
		}

		r := runCLI(t, "", nil, full...)

		if r.Code != ExitUsage {
			t.Errorf("%s: exit %d, want %d\nstderr: %s", name, r.Code, ExitUsage, r.ErrOut)
		}
		if len(api.all()) != 0 {
			t.Errorf("%s: %d request(s) were sent before the mistake was refused", name, len(api.all()))
		}
	}
}

// Scenario: --keep reads an expected refusal like any other page
func TestExpect_KeepReadsTheRefusalPage(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, "-q", "data.0.agentKey", api.URL,
		"register", "email=slow@example.test", "--expect", "429", "--keep", "note=message",
		"then", "verify-email", "email=slow@example.test", "code=654321")

	if r.Code != ExitOK || strings.TrimSpace(r.Out) != "agt_key" {
		t.Fatalf("exit %d, stdout %q, want 0 and agt_key\nstderr: %s", r.Code, r.Out, r.ErrOut)
	}
}
