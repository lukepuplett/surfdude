package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Feature: -v / --trace, one line per step on stderr
//
// Only the last step of a chain prints its response, so a long chain is hard to debug. --trace says
// what each step did, in a line that holds no value, so it is safe to leave on.

// runTraced runs the command with a clock that moves 4ms every time it is read, and counts the reads.
func runTraced(t *testing.T, env map[string]string, args ...string) (result, int) {
	t.Helper()
	var out, errOut bytes.Buffer
	now, reads := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC), 0
	clock := func() time.Time {
		reads++
		at := now
		now = now.Add(4 * time.Millisecond)
		return at
	}
	code := Run(args, IO{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errOut, Now: clock}, func(k string) string { return env[k] })

	return result{Code: code, Out: out.String(), ErrOut: errOut.String()}, reads
}

// traceLines are the lines of stderr that are trace lines.
func traceLines(stderr string) []string {
	var lines []string
	for _, l := range strings.Split(stderr, "\n") {
		if strings.HasPrefix(l, "step ") || strings.HasPrefix(l, "sign-in ") {
			lines = append(lines, l)
		}
	}

	return lines
}

// Scenario: each step of a chain is one line: its number, rel, method, status, time and what it kept
func TestTrace_OneLinePerStep(t *testing.T) {
	api := newFakeAPI(t)

	r, _ := runTraced(t, nil, append([]string{"-v", api.URL}, strings.Fields(sandboxChain)...)...)

	if r.Code != ExitOK {
		t.Fatalf("exit %d\nstderr: %s", r.Code, r.ErrOut)
	}
	want := []string{
		"step 1 register POST 202 4ms",
		"step 2 outbox GET 200 4ms kept=code",
		"step 3 verify-email POST 200 4ms kept=cred",
		"step 4 sign-in POST 200 4ms",
	}
	if got := traceLines(r.ErrOut); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("trace lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Scenario: a trace line holds no value, kept value or secret, only names
func TestTrace_NeverPrintsAValue(t *testing.T) {
	api := newFakeAPI(t)

	r, _ := runTraced(t, nil, append([]string{"-v", api.URL}, strings.Fields(sandboxChain)...)...)

	for _, line := range traceLines(r.ErrOut) {
		for _, value := range []string{"me@example.test", "654321", "agt_testcredential", "tok_signed", "tok_verify", api.URL} {
			if strings.Contains(line, value) {
				t.Errorf("a trace line holds %q: %q", value, line)
			}
		}
	}
}

// Scenario: a step that fails is traced too, with the status it got
func TestTrace_AFailingStepIsTraced(t *testing.T) {
	api := newFakeAPI(t)

	r, _ := runTraced(t, nil, "-v", api.URL, "refuse")

	if r.Code != ExitRefused {
		t.Fatalf("exit %d, want %d", r.Code, ExitRefused)
	}
	if got := traceLines(r.ErrOut); len(got) != 1 || got[0] != "step 1 refuse POST 422 4ms" {
		t.Fatalf("trace lines = %q, want the 422 traced", got)
	}
}

// Scenario: a failure before any step is sent has no step line, because no step ran
func TestTrace_AFailureAtTheFrontDoorHasNoStepLine(t *testing.T) {
	api := newFakeAPI(t)
	url := api.URL
	api.Close() // nothing answers, so even the read of the front door fails

	r, _ := runTraced(t, nil, "-v", url, "refuse")

	if r.Code == ExitOK {
		t.Fatal("a closed server should fail the run")
	}
	if got := traceLines(r.ErrOut); len(got) != 0 {
		t.Fatalf("trace lines before any step ran: %q", got)
	}
}

// Scenario: nothing is traced without the flag, and the clock is not even read
func TestTrace_IsOffByDefault(t *testing.T) {
	api := newFakeAPI(t)

	r, reads := runTraced(t, nil, append([]string{api.URL}, strings.Fields(sandboxChain)...)...)

	if got := traceLines(r.ErrOut); len(got) != 0 {
		t.Fatalf("trace lines without -v: %q", got)
	}
	if reads != 0 {
		t.Fatalf("the clock was read %d time(s) with tracing off", reads)
	}
}

// Scenario: --trace is the same as -v
func TestTrace_LongNameWorksToo(t *testing.T) {
	api := newFakeAPI(t)

	short, _ := runTraced(t, nil, "-v", api.URL, "refuse")
	long, _ := runTraced(t, nil, "--trace", api.URL, "refuse")

	if strings.Join(traceLines(short.ErrOut), "\n") != strings.Join(traceLines(long.ErrOut), "\n") || len(traceLines(long.ErrOut)) != 1 {
		t.Fatalf("-v gave %q, --trace gave %q, want the same one line", traceLines(short.ErrOut), traceLines(long.ErrOut))
	}
}

// Scenario: a sign-in that --sign-in does for a step is traced as its own line
func TestTrace_ASignInIsItsOwnLine(t *testing.T) {
	api := newFakeAPI(t)

	r, _ := runTraced(t, map[string]string{"CRED": "agt_testcredential"}, "-v", "--sign-in", "credential=env:CRED", api.URL, "protected")

	if r.Code != ExitOK {
		t.Fatalf("exit %d\nstderr: %s", r.Code, r.ErrOut)
	}
	joined := strings.Join(traceLines(r.ErrOut), "\n")
	if !strings.Contains(joined, "sign-in POST 200 4ms") {
		t.Errorf("no sign-in line:\n%s", joined)
	}
	if !regexp.MustCompile(`step 1 protected GET 200 \d+ms`).MatchString(joined) {
		t.Errorf("no line for the step itself:\n%s", joined)
	}
}

// Scenario: a script run is traced step by step like a chain
func TestTrace_ApplyToAScript(t *testing.T) {
	api := newFakeAPI(t)
	file := filepath.Join(t.TempDir(), "j.surf")
	script := "root " + api.URL + "\nregister email=me@example.test\n"
	if err := os.WriteFile(file, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}

	r, _ := runTraced(t, nil, "-v", "run", file)

	if got := traceLines(r.ErrOut); len(got) != 1 || got[0] != "step 1 register POST 202 4ms" {
		t.Fatalf("trace lines = %q (exit %d, stderr %s)", got, r.Code, r.ErrOut)
	}
}
