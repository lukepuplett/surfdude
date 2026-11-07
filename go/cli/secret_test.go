package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// Feature: --secret NAME (a value that is asked for, never typed into the command line)

// runWithIO runs the command with a custom IO, for tests of the terminal path.
func runWithIO(t *testing.T, std IO, args ...string) result {
	t.Helper()
	var out, errOut bytes.Buffer
	std.Stdout, std.Stderr = &out, &errOut
	code := Run(args, std, func(string) string { return "" })

	return result{Code: code, Out: out.String(), ErrOut: errOut.String()}
}

// Scenario: not at a terminal, a secret is read from a line of standard input
func TestSecret_ReadsALineOfStdin(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "hunter2-line\n", nil, api.URL, "set-password", "email=a@b.test", "--secret", "password")

	if r.Code != ExitOK || api.last().Body != `{"email":"a@b.test","password":"hunter2-line"}` {
		t.Fatalf("exit %d, body %q\nstderr: %s", r.Code, api.last().Body, r.ErrOut)
	}
	if strings.Contains(r.Out, "hunter2-line") || strings.Contains(r.ErrOut, "hunter2-line") {
		t.Fatalf("the secret was printed:\nstdout: %s\nstderr: %s", r.Out, r.ErrOut)
	}
}

// Scenario: several secrets come from successive lines, in the order they were named
func TestSecret_SeveralComeFromSuccessiveLines(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "a@b.test\nsecond-secret\n", nil, api.URL, "set-password", "--secret", "email", "--secret", "password")

	if r.Code != ExitOK || api.last().Body != `{"email":"a@b.test","password":"second-secret"}` {
		t.Fatalf("exit %d, body %q\nstderr: %s", r.Code, api.last().Body, r.ErrOut)
	}
}

// Scenario: at a terminal, the secret is asked for with the typing hidden, and named in the prompt
func TestSecret_AtATerminalItIsPromptedForWithTheTypingHidden(t *testing.T) {
	api := newFakeAPI(t)
	var prompts []string

	r := runWithIO(t, IO{
		Stdin:           strings.NewReader(""),
		StdinIsTerminal: true,
		ReadSecret:      func(p string) (string, error) { prompts = append(prompts, p); return "typed-at-prompt", nil },
	}, api.URL, "set-password", "email=a@b.test", "--secret", "password")

	if r.Code != ExitOK || api.last().Body != `{"email":"a@b.test","password":"typed-at-prompt"}` {
		t.Fatalf("exit %d, body %q\nstderr: %s", r.Code, api.last().Body, r.ErrOut)
	}
	if len(prompts) != 1 || !strings.Contains(prompts[0], "password") {
		t.Fatalf("prompts = %q, want one that names the field", prompts)
	}
	if strings.Contains(r.Out+r.ErrOut, "typed-at-prompt") {
		t.Fatalf("the typed secret was printed:\n%s%s", r.Out, r.ErrOut)
	}
}

// Scenario: at a terminal that cannot hide typing, the command refuses rather than show it
func TestSecret_RefusesWhenTypingCannotBeHidden(t *testing.T) {
	api := newFakeAPI(t)

	r := runWithIO(t, IO{Stdin: strings.NewReader(""), StdinIsTerminal: true},
		api.URL, "set-password", "email=a@b.test", "--secret", "password")

	if r.Code != ExitUsage || !strings.Contains(r.ErrOut, "standard input") {
		t.Fatalf("exit %d, stderr %q, want 2 and the advice to pipe it in", r.Code, r.ErrOut)
	}
	if api.count("POST", "/password") != 0 {
		t.Fatal("a request was sent")
	}
}

// Scenario: a prompt that fails is a usage error, and nothing is sent
func TestSecret_APromptThatFailsStopsTheRun(t *testing.T) {
	api := newFakeAPI(t)

	r := runWithIO(t, IO{
		Stdin: strings.NewReader(""), StdinIsTerminal: true,
		ReadSecret: func(string) (string, error) { return "", errors.New("interrupted") },
	}, api.URL, "set-password", "email=a@b.test", "--secret", "password")

	if r.Code != ExitUsage || api.count("POST", "/password") != 0 {
		t.Fatalf("exit %d, %d POSTs, want 2 and none", r.Code, api.count("POST", "/password"))
	}
}

// Scenario: standard input with nothing in it is an error, not an empty secret
func TestSecret_EmptyStdinIsAnError(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, api.URL, "set-password", "email=a@b.test", "--secret", "password")

	if r.Code != ExitUsage || api.count("POST", "/password") != 0 {
		t.Fatalf("exit %d, %d POSTs, want 2 and none\nstderr: %s", r.Code, api.count("POST", "/password"), r.ErrOut)
	}
}

// Scenario: a field named with --secret is hidden in a dry run even when its name is not secret-looking
func TestSecret_IsHiddenInADryRunWhateverItsName(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "private@b.test\npw\n", nil, "--dry-run", api.URL, "set-password", "--secret", "email", "--secret", "password")

	if r.Code != ExitOK || strings.Contains(r.Out, "private@b.test") || !strings.Contains(r.Out, `"email":"<secret>"`) {
		t.Fatalf("exit %d, the email given with --secret was shown, or not masked:\n%s", r.Code, r.Out)
	}
}

// Scenario: a field that the form does not have is refused even when it is a secret
func TestSecret_UnknownFieldIsRefusedWithoutPrompting(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "whatever\n", nil, api.URL, "register", "email=a@b.test", "--secret", "pin")

	if r.Code != ExitUsage || !strings.Contains(r.ErrOut, "pin") || api.count("POST", "/auth/email/start") != 0 {
		t.Fatalf("exit %d, stderr %q", r.Code, r.ErrOut)
	}
}
