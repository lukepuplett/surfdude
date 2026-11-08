package cli

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Feature: --dry-run shows the request and sends nothing, with secrets hidden

// Scenario: the request is shown in full and not sent
func TestDryRun_ShowsTheRequestAndSendsNothing(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, "--dry-run", api.URL, "register", "email=me@example.test")

	if r.Code != ExitOK {
		t.Fatalf("exit %d\nstderr: %s", r.Code, r.ErrOut)
	}
	for _, want := range []string{
		"POST register -> " + api.URL + "/auth/email/start",
		"Content-Type: application/json",
		`body: {"email":"me@example.test"}`,
	} {
		if !strings.Contains(r.Out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.Out)
		}
	}
	if n := api.count("POST", "/auth/email/start"); n != 0 {
		t.Fatalf("the dry run sent the request %d time(s)", n)
	}
	if len(api.all()) != 1 || api.all()[0].Path != "/" {
		t.Fatalf("requests = %+v, want only the read of the front door", api.all())
	}
}

// Scenario: every way a secret can reach a request is hidden in the dry run
func TestDryRun_HidesSecrets(t *testing.T) {
	api := newFakeAPI(t)
	env := map[string]string{"PW": "hunter2-from-env"}

	cases := []struct {
		name   string
		args   []string
		stdin  string
		secret string
	}{
		{"from the environment", []string{"set-password", "email=a@b.test", "password=env:PW"}, "", "hunter2-from-env"},
		{"from standard input", []string{"set-password", "email=a@b.test", "password=@-"}, "stdin-secret-77\n", "stdin-secret-77"},
		{"typed on the command line under a secret-looking name", []string{"set-password", "email=a@b.test", "password=typed-secret-55"}, "", "typed-secret-55"},
	}

	for _, c := range cases {
		r := runCLI(t, c.stdin, env, append([]string{"--dry-run", api.URL}, c.args...)...)

		if r.Code != ExitOK {
			t.Errorf("%s: exit %d\nstderr: %s", c.name, r.Code, r.ErrOut)
			continue
		}
		if strings.Contains(r.Out, c.secret) || strings.Contains(r.ErrOut, c.secret) {
			t.Errorf("%s: the secret was printed:\nstdout: %s\nstderr: %s", c.name, r.Out, r.ErrOut)
		}
		if !strings.Contains(r.Out, `"password":"<secret>"`) {
			t.Errorf("%s: the body does not show that the password was hidden:\n%s", c.name, r.Out)
		}
		if !strings.Contains(r.Out, `"email":"a@b.test"`) {
			t.Errorf("%s: the non-secret field was hidden too:\n%s", c.name, r.Out)
		}
	}

	r := runCLI(t, "", nil, "--dry-run", "--bearer", "bearer-sekrit-9", api.URL, "set-password", "email=a@b.test", "password=x")
	if strings.Contains(r.Out, "bearer-sekrit-9") || strings.Contains(r.ErrOut, "bearer-sekrit-9") {
		t.Fatalf("the bearer token was printed:\n%s", r.Out)
	}
	if !strings.Contains(r.Out, "Authorization: Bearer <secret>") {
		t.Fatalf("the plan does not show that a token would be sent:\n%s", r.Out)
	}
}

// Scenario: a secret in the query string of a read is hidden too
func TestDryRun_HidesSecretsInTheQuery(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, "--dry-run", api.URL, "peek", "token=sekrit123", "to=x@y.test")

	if strings.Contains(r.Out, "sekrit123") || !strings.Contains(r.Out, "token=%3Csecret%3E") || !strings.Contains(r.Out, "to=x%40y.test") {
		t.Fatalf("the query is not masked as expected:\n%s", r.Out)
	}
}

// Scenario: a value read from a file shows where it came from, not what is in it
func TestDryRun_ShowsWhereAFileValueCameFrom(t *testing.T) {
	api := newFakeAPI(t)
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("private doc"), 0o600); err != nil {
		t.Fatal(err)
	}

	r := runCLI(t, "", nil, "--dry-run", api.URL, "set-count", "count:=1", "note=@"+path)

	if r.Code != ExitOK || strings.Contains(r.Out, "private doc") || !strings.Contains(r.Out, "<@"+path+">") {
		t.Fatalf("exit %d: the file's content was shown, or its source was not:\nstdout: %s\nstderr: %s", r.Code, r.Out, r.ErrOut)
	}
}

// Scenario: for a chain, only the first step can be shown, because the rest depend on responses
func TestDryRun_ChainShowsTheFirstStepAndNamesTheRest(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, append([]string{"--dry-run", api.URL}, strings.Fields(sandboxChain)...)...)

	if r.Code != ExitOK || !strings.Contains(r.Out, "POST register ->") {
		t.Fatalf("exit %d, stdout:\n%s", r.Code, r.Out)
	}
	for _, rel := range []string{"outbox", "verify-email", "sign-in"} {
		if !strings.Contains(r.Out, "then "+rel+": not planned") {
			t.Errorf("stdout does not say that %s is not planned:\n%s", rel, r.Out)
		}
	}
	if len(api.all()) != 1 {
		t.Fatalf("%d requests were sent, want only the read of the front door", len(api.all()))
	}
}

// Scenario: the dry run refuses what the real run would refuse, with the same message
func TestDryRun_RefusesWhatARealRunRefuses(t *testing.T) {
	api := newFakeAPI(t)

	if r := runCLI(t, "", nil, "--dry-run", api.URL, "register"); r.Code != ExitUsage || !strings.Contains(r.ErrOut, "email") {
		t.Fatalf("exit %d, stderr %q, want 2 naming the missing field", r.Code, r.ErrOut)
	}
}

// Scenario: a value built from a secret that an earlier step kept is a secret too
func TestBuildValues_MasksAValueBuiltFromAKeptSecret(t *testing.T) {
	vals := []value{
		{Name: "auth", V: "Bearer {cred}", Template: true},
		{Name: "email", V: "a@b.test", Template: true},
		{Name: "doc", V: "contents", File: "notes.txt"},
	}
	kept := map[string]keptValue{"cred": {Text: "agt_x", Secret: true}}

	sent, mask, err := buildValues(vals, kept)

	if err != nil {
		t.Fatal(err)
	}
	if sent["auth"] != "Bearer agt_x" || sent["email"] != "a@b.test" || sent["doc"] != "contents" {
		t.Fatalf("sent = %+v, want the real values to be sent", sent)
	}
	if mask["auth"] != "<secret>" {
		t.Errorf("mask[auth] = %q, want <secret>, because a kept secret went into it", mask["auth"])
	}
	if _, masked := mask["email"]; masked {
		t.Errorf("a plain value was masked: %q", mask["email"])
	}
	if mask["doc"] != "<@notes.txt>" {
		t.Errorf("mask[doc] = %q, want the file it came from", mask["doc"])
	}
}

// Scenario: the address shown in an announcement never carries a secret from the query
func TestMaskedURL(t *testing.T) {
	u, _ := url.Parse("https://x.test/lookup?token=abc123&to=a%40b.test&note=plain")
	mask := map[string]string{"note": "<@notes.txt>"}

	got := maskedURL(u, mask, masking{})

	if strings.Contains(got, "abc123") || !strings.Contains(got, "to=a%40b.test") || !strings.Contains(got, "note=%3C%40notes.txt%3E") {
		t.Fatalf("maskedURL = %s", got)
	}
	if want := "https://x.test/lookup"; !strings.HasPrefix(got, want) {
		t.Fatalf("maskedURL = %s, want it to start with %s", got, want)
	}
}

func TestMaskBody(t *testing.T) {
	mask := map[string]string{"location.lat": "<secret>"}

	if got := maskBody("application/json", []byte(`{"email":"a@b.test","location":{"lat":51.5,"lon":-0.1},"access_token":"zzz","authTokenHint":"visible"}`), mask, masking{}); got !=
		`{"access_token":"<secret>","authTokenHint":"visible","email":"a@b.test","location":{"lat":"<secret>","lon":-0.1}}` {
		t.Errorf("JSON body = %s", got)
	}
	if got := maskBody("application/x-www-form-urlencoded", []byte(`email=a%40b.test&password=pw`), nil, masking{}); got != `email=a%40b.test&password=%3Csecret%3E` {
		t.Errorf("form body = %s", got)
	}
	if got := maskBody("text/plain", []byte(`just text`), nil, masking{}); got != "<body of 9 bytes, text/plain>" {
		t.Errorf("other body = %s", got)
	}
}
