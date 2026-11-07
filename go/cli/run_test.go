package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// Feature: surf ROOT REL name=value (one step from the front door)

// Scenario: a step finds the control by its rel, fills it and sends it
func TestRun_OneShotSendsTheForm(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, api.URL, "register", "email=me@example.test")

	if r.Code != ExitOK {
		t.Fatalf("exit %d, want 0\nstdout: %s\nstderr: %s", r.Code, r.Out, r.ErrOut)
	}
	got := api.last()
	if got.Method != "POST" || got.Path != "/auth/email/start" || got.ContentType != "application/json" || got.Body != `{"email":"me@example.test"}` {
		t.Fatalf("the server received %+v", got)
	}
	for _, want := range []string{"202 Accepted", "note: A code is on its way.", "next:", "verify-email"} {
		if !strings.Contains(r.Out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.Out)
		}
	}
}

// Scenario: anything that is not a GET says what it is about to do, on stderr, before sending
func TestRun_NonGetAnnouncesItself(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, api.URL, "register", "email=me@example.test")

	if want := "POST register -> " + api.URL + "/auth/email/start"; !strings.Contains(r.ErrOut, want) {
		t.Fatalf("stderr = %q, want it to say %q", r.ErrOut, want)
	}
}

// Scenario: a read is quiet, and a link with a URI template takes its values from the step
func TestRun_GetStepIsQuietAndFillsTheTemplate(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, api.URL, "outbox", "to=me@example.test")

	if r.Code != ExitOK || r.ErrOut != "" {
		t.Fatalf("exit %d, stderr %q, want 0 and nothing on stderr for a GET", r.Code, r.ErrOut)
	}
	if got := api.last(); got.Method != "GET" || got.Path != "/sandbox/outbox" || got.Query != "to=me%40example.test" {
		t.Fatalf("the server received %+v", got)
	}
	if !strings.Contains(r.Out, "Your code is 654321") {
		t.Fatalf("stdout does not show the data:\n%s", r.Out)
	}
}

// Scenario: --json prints the whole envelope and nothing else on stdout
func TestRun_JSONPrintsTheEnvelope(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, "--json", api.URL, "register", "email=me@example.test")

	var env struct{ Status int }
	if err := json.Unmarshal([]byte(r.Out), &env); err != nil || env.Status != 202 {
		t.Fatalf("stdout is not the envelope (%v):\n%s", err, r.Out)
	}
}

// Scenario: -q prints one value, plain, so it works inside $(...)
func TestRun_QPrintsOneValue(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, "-q", "data.0.text", api.URL, "outbox", "to=me@example.test")

	if r.Code != ExitOK || r.Out != "Your code is 654321\n" {
		t.Fatalf("exit %d, stdout %q, want exactly the value and a newline", r.Code, r.Out)
	}
}

// Scenario: -q with a path that leads nowhere is a usage error, with nothing on stdout
func TestRun_QWithAMissingPathIsAUsageError(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, "-q", "data.0.nope", api.URL, "outbox", "to=a@b.test")

	if r.Code != ExitUsage || r.Out != "" || !strings.Contains(r.ErrOut, "data.0.nope") {
		t.Fatalf("exit %d, stdout %q, stderr %q, want 2, nothing, and the path named", r.Code, r.Out, r.ErrOut)
	}
}

// Scenario: --json and -q together are ambiguous
func TestRun_JSONAndQTogetherIsAUsageError(t *testing.T) {
	api := newFakeAPI(t)

	if r := runCLI(t, "", nil, "--json", "-q", "status", api.URL, "outbox", "to=a@b.test"); r.Code != ExitUsage {
		t.Fatalf("exit %d, want 2", r.Code)
	}
}

// Scenario: the server refusing the request exits 1, shows why on stderr and prints nothing on stdout
func TestRun_ServerRefusalExits1(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, api.URL, "refuse")

	if r.Code != ExitRefused || r.Out != "" {
		t.Fatalf("exit %d, stdout %q, want 1 and an empty stdout", r.Code, r.Out)
	}
	for _, want := range []string{"422", "Unable to refuse. Not allowed.", "email: Use a real address.", "note: Try something else.", "retry after: 90"} {
		if !strings.Contains(r.ErrOut, want) {
			t.Errorf("stderr lacks %q:\n%s", want, r.ErrOut)
		}
	}
}

// Scenario: with --json a refusal still puts the envelope on stdout, so a script can read the problems
func TestRun_JSONRefusalKeepsTheEnvelopeOnStdout(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, "--json", api.URL, "refuse")

	var env struct{ Status int }
	if r.Code != ExitRefused || json.Unmarshal([]byte(r.Out), &env) != nil || env.Status != 422 {
		t.Fatalf("exit %d, stdout %q, want 1 and the 422 envelope", r.Code, r.Out)
	}
}

// Scenario: the server failing exits 4
func TestRun_ServerFailureExits4(t *testing.T) {
	api := newFakeAPI(t)

	if r := runCLI(t, "", nil, api.URL, "boom"); r.Code != ExitServer {
		t.Fatalf("exit %d, want 4\nstderr: %s", r.Code, r.ErrOut)
	}
}

// Scenario: not reaching the server exits 3
func TestRun_NetworkFailureExits3(t *testing.T) {
	if r := runCLI(t, "", nil, "http://127.0.0.1:1", "register", "email=a@b.test"); r.Code != ExitNetwork {
		t.Fatalf("exit %d, want 3\nstderr: %s", r.Code, r.ErrOut)
	}
}

// Scenario: a mistake in the command is refused before anything is sent, and says what is accepted
func TestRun_UsageErrorsSendNothingAndSayWhatIsAccepted(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string // something the message must contain
	}{
		{"a missing required field", []string{"register"}, "email"},
		{"a field the form does not have", []string{"register", "email=a@b.test", "colour=red"}, "colour"},
		{"a rel the server does not offer", []string{"nope"}, "register"},
		{"text where a number belongs", []string{"set-count", "count=abc"}, "count:="},
		{"a number out of range", []string{"set-count", "count:=99"}, "at most 10"},
		{"something that is not an email", []string{"register", "email=not an address"}, "email"},
	}

	for _, c := range cases {
		api := newFakeAPI(t)

		r := runCLI(t, "", nil, append([]string{api.URL}, c.args...)...)

		if r.Code != ExitUsage {
			t.Errorf("%s: exit %d, want 2\nstderr: %s", c.name, r.Code, r.ErrOut)
		}
		if !strings.Contains(r.ErrOut, c.want) {
			t.Errorf("%s: stderr %q does not mention %q", c.name, r.ErrOut, c.want)
		}
		if posts := len(api.all()) - api.count("GET", "/"); posts != 0 {
			t.Errorf("%s: %d request(s) beyond the front door were sent, want none", c.name, posts)
		}
	}
}

// Scenario: typed values go to the server as their type
func TestRun_TypedValuesAreSentAsTheirType(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, api.URL, "set-count", "count:=3", "note=hi")

	if r.Code != ExitOK || api.last().Body != `{"count":3,"note":"hi"}` {
		t.Fatalf("exit %d, body %q, want {\"count\":3,\"note\":\"hi\"}\nstderr: %s", r.Code, api.last().Body, r.ErrOut)
	}
}

// Scenario: --bearer and --bearer-env send the token
func TestRun_BearerIsSent(t *testing.T) {
	api := newFakeAPI(t)

	runCLI(t, "", nil, "--bearer", "abc", api.URL, "protected")
	if got := api.last().Auth; got != "Bearer abc" {
		t.Errorf("--bearer sent %q", got)
	}
	runCLI(t, "", map[string]string{"AT_TOKEN": "fromenv"}, "--bearer-env", "AT_TOKEN", api.URL, "protected")
	if got := api.last().Auth; got != "Bearer fromenv" {
		t.Errorf("--bearer-env sent %q", got)
	}
	if r := runCLI(t, "", nil, "--bearer-env", "UNSET_VAR", api.URL, "protected"); r.Code != ExitUsage {
		t.Errorf("an unset --bearer-env exit %d, want 2", r.Code)
	}
	if r := runCLI(t, "", map[string]string{"X": "y"}, "--bearer", "a", "--bearer-env", "X", api.URL, "protected"); r.Code != ExitUsage {
		t.Errorf("--bearer with --bearer-env exit %d, want 2", r.Code)
	}
}

// Scenario: a root without a scheme is https, except on this machine, where it is http
func TestNormaliseRoot(t *testing.T) {
	for in, want := range map[string]string{
		"agenttrader.net":       "https://agenttrader.net",
		"http://localhost:8080": "http://localhost:8080",
		"https://x.test/api":    "https://x.test/api",
		"localhost:8080":        "http://localhost:8080",
		"127.0.0.1:9000/start":  "http://127.0.0.1:9000/start",
		"[::1]:8080":            "http://[::1]:8080",
		"example.com:8443/api":  "https://example.com:8443/api",
	} {
		if got := normaliseRoot(in); got != want {
			t.Errorf("normaliseRoot(%q) = %q, want %q", in, got, want)
		}
	}
}

// Scenario: no arguments is a usage error that shows how to use the command
func TestRun_NoArgumentsIsUsage(t *testing.T) {
	r := runCLI(t, "", nil)

	if r.Code != ExitUsage || !strings.Contains(r.ErrOut, "usage:") {
		t.Fatalf("exit %d, stderr %q, want 2 and a usage line", r.Code, r.ErrOut)
	}
}
