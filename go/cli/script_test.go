package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Feature: surf run FILE (a journey written down: one step per line)

func writeScript(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "journey.surf")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

// Scenario: a script is a chain, with the root on its first line, one step per line, and comments
func TestRunScript_RunsEachLineAsAStep(t *testing.T) {
	api := newFakeAPI(t)
	path := writeScript(t, "# register, read the code, verify, sign in\n"+
		"root "+api.URL+"\n"+
		"\n"+
		"register email=me@example.test\r\n"+
		"outbox to=me@example.test --keep \"code=data.0.text~[0-9]{6}\"\n"+
		"verify-email email=me@example.test code={code} --keep cred=data.0.credential\n"+
		"   # an indented comment\n"+
		"sign-in credential={cred}\n")

	r := runCLI(t, "", nil, "-q", "data.0.token", "run", path)

	if r.Code != ExitOK || r.Out != "tok_signed\n" {
		t.Fatalf("exit %d, stdout %q, want 0 and tok_signed\nstderr: %s", r.Code, r.Out, r.ErrOut)
	}
	if api.count("POST", "/auth/email/start") != 1 || api.count("POST", "/auth/token") != 1 {
		t.Fatalf("requests = %+v", api.all())
	}
}

// Scenario: quotes work in a script line, and a line may hold more than one step
func TestRunScript_QuotesAndThenWithinALine(t *testing.T) {
	api := newFakeAPI(t)
	path := writeScript(t, "root "+api.URL+"\n"+
		"register email=a@b.test then set-count count:=1 note=\"two words\"\n")

	// set-count is only on the front door, and this test is about quotes, not about finding steps.
	r := runCLI(t, "", nil, "--front-door", "run", path)

	if r.Code != ExitOK || !strings.Contains(api.last().Body, `"note":"two words"`) {
		t.Fatalf("exit %d, body %q\nstderr: %s", r.Code, api.last().Body, r.ErrOut)
	}
}

// Scenario: flags before run apply to the script, such as --json
func TestRunScript_FlagsApply(t *testing.T) {
	api := newFakeAPI(t)
	path := writeScript(t, "root "+api.URL+"\nregister email=a@b.test\n")

	r := runCLI(t, "", nil, "--json", "run", path)

	if r.Code != ExitOK || !strings.HasPrefix(r.Out, `{"status":202`) {
		t.Fatalf("exit %d, stdout %q, want the 202 envelope", r.Code, r.Out)
	}
}

// Scenario: a dry run of a script shows the first step and names the rest
func TestRunScript_DryRun(t *testing.T) {
	api := newFakeAPI(t)
	path := writeScript(t, "root "+api.URL+"\nregister email=a@b.test\nset-count count:=2\n")

	r := runCLI(t, "", nil, "--dry-run", "run", path)

	if r.Code != ExitOK || !strings.Contains(r.Out, "POST register ->") || !strings.Contains(r.Out, "then set-count: not planned") {
		t.Fatalf("exit %d, stdout:\n%s", r.Code, r.Out)
	}
	if api.count("POST", "/auth/email/start") != 0 {
		t.Fatal("the dry run sent a request")
	}
}

// Scenario: a script that cannot be read is a usage error, with the line it went wrong on, and nothing is sent
func TestRunScript_Refusals(t *testing.T) {
	api := newFakeAPI(t)
	cases := map[string]struct {
		content string
		want    string
	}{
		"no root":                {"register email=a@b.test\n", "root"},
		"root has no address":    {"root\nregister email=a@b.test\n", "root"},
		"root with two words":    {"root " + api.URL + " extra\nregister\n", "root"},
		"nothing after the root": {"root " + api.URL + "\n# nothing\n", "no steps"},
		"an empty file":          {"", "root"},
		"a bad line says which":  {"root " + api.URL + "\nregister email=a@b.test\nregister email=\"open\n", "line 3"},
		"a bad chain says which": {"root " + api.URL + "\nregister email=a@b.test then then x\n", "line 2"},
		"an unknown template":    {"root " + api.URL + "\nregister email={nobody}\n", "{nobody}"},
	}

	for name, c := range cases {
		before := len(api.all())

		r := runCLI(t, "", nil, "run", writeScript(t, c.content))

		if r.Code != ExitUsage || !strings.Contains(r.ErrOut, c.want) {
			t.Errorf("%s: exit %d, stderr %q, want 2 mentioning %q", name, r.Code, r.ErrOut, c.want)
		}
		if len(api.all()) != before {
			t.Errorf("%s: a request was sent", name)
		}
	}
}

func TestRunScript_NeedsAFile(t *testing.T) {
	if r := runCLI(t, "", nil, "run"); r.Code != ExitUsage {
		t.Errorf("no file: exit %d, want 2", r.Code)
	}
	if r := runCLI(t, "", nil, "run", filepath.Join(t.TempDir(), "missing.surf")); r.Code != ExitUsage || !strings.Contains(r.ErrOut, "missing.surf") {
		t.Errorf("a missing file: exit %d, stderr %q, want 2 naming it", r.Code, r.ErrOut)
	}
}
