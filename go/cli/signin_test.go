package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// Feature: --sign-in 'name=value ...' (a 401 is the instruction to sign in, once)

// Scenario: a step refused with 401 signs in by the sign-in rel and is sent again with the token
func TestSignIn_AStepRefusedWith401SignsInAndRetriesOnce(t *testing.T) {
	api := newFakeAPI(t)
	env := map[string]string{"CRED": "agt_testcredential"}

	r := runCLI(t, "", env, "--sign-in", "credential=env:CRED", api.URL, "protected")

	if r.Code != ExitOK || !strings.Contains(r.Out, "hello=you") {
		t.Fatalf("exit %d, stdout %q\nstderr: %s", r.Code, r.Out, r.ErrOut)
	}
	var seq []string
	for _, q := range api.all() {
		seq = append(seq, q.Method+" "+q.Path)
	}
	if want := "GET / | GET /protected | POST /auth/token | GET /protected"; strings.Join(seq, " | ") != want {
		t.Fatalf("requests = %s\nwant       %s", strings.Join(seq, " | "), want)
	}
	if got := api.last().Auth; got != "Bearer tok_signed" {
		t.Fatalf("the retry carried %q, want the token from sign-in", got)
	}
	if !strings.Contains(r.ErrOut, "POST sign-in -> ") {
		t.Errorf("the sign-in did not announce itself:\n%s", r.ErrOut)
	}
	if strings.Contains(r.Out+r.ErrOut, "agt_testcredential") || strings.Contains(r.Out+r.ErrOut, "tok_signed") {
		t.Fatalf("a secret was printed:\nstdout: %s\nstderr: %s", r.Out, r.ErrOut)
	}
}

// Scenario: nothing is signed in unless a step is refused
func TestSignIn_NotUsedWhenNothingIsRefused(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", map[string]string{"CRED": "x"}, "--sign-in", "credential=env:CRED", api.URL, "outbox", "to=a@b.test")

	if r.Code != ExitOK || api.count("POST", "/auth/token") != 0 {
		t.Fatalf("exit %d, %d sign-ins, want 0 and none", r.Code, api.count("POST", "/auth/token"))
	}
}

// Scenario: a wrong credential is the server's refusal, exit 1, and says that signing in failed
func TestSignIn_AWrongCredentialExits1AndSaysSo(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", map[string]string{"CRED": "agt_wrong_value"}, "--sign-in", "credential=env:CRED", api.URL, "protected")

	if r.Code != ExitRefused || !strings.Contains(r.ErrOut, "Signing in again") {
		t.Fatalf("exit %d, stderr %q, want 1 and a message that signing in failed", r.Code, r.ErrOut)
	}
	if strings.Contains(r.Out+r.ErrOut, "agt_wrong_value") {
		t.Fatalf("the credential was printed:\n%s%s", r.Out, r.ErrOut)
	}
	if api.count("GET", "/protected") != 1 {
		t.Fatalf("the step was sent %d times, want once, because sign-in failed", api.count("GET", "/protected"))
	}
}

// bareAPI offers one protected route that always refuses and no way to sign in.
func bareAPI(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Add("Link", profileLink)
		if r.URL.Path == "/protected" {
			w.WriteHeader(401)
			_, _ = io.WriteString(w, `{"status":401,"message":"Sign in","data":[]}`)
			return
		}
		_, _ = io.WriteString(w, `{"status":200,"message":"OK","data":[],"_links":[{"rel":"protected","href":"/protected","method":"GET"}]}`)
	}))
	t.Cleanup(srv.Close)

	return srv
}

// Scenario: a server that offers no sign-in is a usage error for --sign-in, not a network failure
func TestSignIn_NoSignInRelIsAUsageError(t *testing.T) {
	srv := bareAPI(t)

	r := runCLI(t, "", map[string]string{"CRED": "x"}, "--sign-in", "credential=env:CRED", srv.URL, "protected")

	if r.Code != ExitUsage || !strings.Contains(r.ErrOut, "sign-in") {
		t.Fatalf("exit %d, stderr %q, want 2 naming the missing sign-in", r.Code, r.ErrOut)
	}
}

// Scenario: --token-path says where in the sign-in response the token is
func TestSignIn_TokenPath(t *testing.T) {
	api := newFakeAPI(t)

	// agentKey is the wrong thing to send, but it proves the path was used.
	runCLI(t, "", map[string]string{"CRED": "agt_testcredential"}, "--sign-in", "credential=env:CRED", "--token-path", "data.0.agentKey", api.URL, "protected")

	if got := api.last().Auth; got != "Bearer agt_key" {
		t.Fatalf("the retry carried %q, want the value at --token-path", got)
	}
	r := runCLI(t, "", map[string]string{"CRED": "agt_testcredential"}, "--sign-in", "credential=env:CRED", "--token-path", "data.0.nope", api.URL, "protected")
	if r.Code == ExitOK {
		t.Fatal("a --token-path that finds nothing was treated as a token")
	}
}

// Scenario: values for the sign-in follow the same syntax, and are checked before anything is sent
func TestSignIn_ValuesAreCheckedUpFront(t *testing.T) {
	for name, signIn := range map[string]string{
		"a word that is not name=value":            "credential",
		"a template, which has nothing to fill it": "credential={cred}",
		"an unterminated quote":                    `credential="abc`,
	} {
		api := newFakeAPI(t)

		r := runCLI(t, "", nil, "--sign-in", signIn, api.URL, "protected")

		if r.Code != ExitUsage || len(api.all()) != 0 {
			t.Errorf("%s: exit %d, %d requests, want 2 and none\nstderr: %s", name, r.Code, len(api.all()), r.ErrOut)
		}
	}
}

// Scenario: standard input is read once, even across a step and the sign-in
func TestSignIn_StdinCannotFeedTheStepAndTheSignIn(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "agt_testcredential\n", nil, "--sign-in", "credential=@-", api.URL, "set-password", "email=a@b.test", "password=@-")

	if r.Code != ExitUsage || len(api.all()) != 0 {
		t.Fatalf("exit %d, %d requests, want 2 and none\nstderr: %s", r.Code, len(api.all()), r.ErrOut)
	}
}

func TestSplitWords(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"a=1 b=2", []string{"a=1", "b=2"}},
		{`note="hello world" x=1`, []string{"note=hello world", "x=1"}},
		{`note='it''s fine'`, []string{"note=its fine"}},
		{`a="say \"hi\"" b=c`, []string{`a=say "hi"`, "b=c"}},
		{`a=one\ two`, []string{"a=one two"}},
		{`a='keeps \ and "quotes"'`, []string{`a=keeps \ and "quotes"`}},
		{`a=""`, []string{"a="}},
	}

	for _, c := range cases {
		got, err := splitWords(c.in)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitWords(%q) = %q, %v, want %q", c.in, got, err, c.want)
		}
	}
	for _, bad := range []string{`a="open`, `a='open`, `a=trailing\`} {
		if _, err := splitWords(bad); err == nil {
			t.Errorf("splitWords(%q) was accepted", bad)
		}
	}
}
