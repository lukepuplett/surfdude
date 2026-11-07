package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Feature: the value syntax: name=text, name:=JSON, name=@file, name=@-, name=env:VAR

func resolverWith(stdin string, env map[string]string) *resolver {
	return &resolver{
		env:   func(k string) string { return env[k] },
		stdin: bufio.NewReader(strings.NewReader(stdin)),
	}
}

// Scenario: each way of writing a value is understood
func TestParseArg_Forms(t *testing.T) {
	cases := []struct {
		tok  string
		name string
		kind argKind
		raw  string
	}{
		{"email=me@example.test", "email", kindText, "me@example.test"},
		{"note=a=b=c", "note", kindText, "a=b=c"}, // only the first = splits
		{"empty=", "empty", kindText, ""},
		{"count:=42", "count", kindJSON, "42"},
		{`where:={"lat":1.5}`, "where", kindJSON, `{"lat":1.5}`},
		{"body=@notes.txt", "body", kindFile, "notes.txt"},
		{"credential=@-", "credential", kindStdin, ""},
		{"credential=env:AT_CRED", "credential", kindEnv, "AT_CRED"},
		{"location.lat:=51.5", "location.lat", kindJSON, "51.5"}, // dotted names nest in the body
	}

	for _, c := range cases {
		got, err := parseArg(c.tok)
		if err != nil {
			t.Errorf("parseArg(%q): %v", c.tok, err)
			continue
		}
		if got.Name != c.name || got.Kind != c.kind || got.Raw != c.raw {
			t.Errorf("parseArg(%q) = %+v, want name %q kind %v raw %q", c.tok, got, c.name, c.kind, c.raw)
		}
	}
}

// Scenario: something that is not a value is a usage error that says what was expected
func TestParseArg_Refusals(t *testing.T) {
	for _, tok := range []string{"justaword", "=novalue", "has space=x", ":=1", "a:b=c", "-flagish"} {
		_, err := parseArg(tok)
		var u *usageError
		if !errors.As(err, &u) {
			t.Errorf("parseArg(%q) err = %v, want a usage error", tok, err)
		}
	}
}

// Scenario: text stays text, JSON becomes its type, and a number keeps its exact digits
func TestResolve_TextAndJSON(t *testing.T) {
	r := resolverWith("", nil)

	text, _ := r.resolve(arg{Name: "count", Kind: kindText, Raw: "42"})
	if text.V != "42" {
		t.Errorf("name=42 gave %#v, want the string 42", text.V)
	}
	num, err := r.resolve(arg{Name: "count", Kind: kindJSON, Raw: "12345678901234567890"})
	if err != nil || num.V != json.Number("12345678901234567890") {
		t.Errorf("count:=12345678901234567890 gave %#v, %v, want the exact number", num.V, err)
	}
	flag, _ := r.resolve(arg{Name: "ok", Kind: kindJSON, Raw: "true"})
	if flag.V != true {
		t.Errorf("ok:=true gave %#v, want a boolean", flag.V)
	}
	obj, _ := r.resolve(arg{Name: "w", Kind: kindJSON, Raw: `{"a":1}`})
	if m, ok := obj.V.(map[string]any); !ok || m["a"] != json.Number("1") {
		t.Errorf("w:={\"a\":1} gave %#v, want an object", obj.V)
	}
	for _, bad := range []string{"{", "", "1 2", "nope"} {
		if _, err := r.resolve(arg{Name: "x", Kind: kindJSON, Raw: bad}); err == nil {
			t.Errorf("x:=%q was accepted as JSON", bad)
		}
	}
}

// Scenario: a value can come from the environment, and a missing one is an error not an empty string
func TestResolve_Env(t *testing.T) {
	r := resolverWith("", map[string]string{"AT_CRED": "agt_secret"})

	got, err := r.resolve(arg{Name: "credential", Kind: kindEnv, Raw: "AT_CRED"})
	if err != nil || got.V != "agt_secret" || !got.Secret {
		t.Fatalf("env value = %#v, %v, want agt_secret marked secret", got, err)
	}
	if _, err := r.resolve(arg{Name: "credential", Kind: kindEnv, Raw: "NOT_SET"}); err == nil {
		t.Fatal("an unset environment variable gave a value, so a typo would send an empty credential")
	}
}

// Scenario: a value can come from a file, with one trailing newline removed
func TestResolve_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := resolverWith("", nil)

	got, err := r.resolve(arg{Name: "body", Kind: kindFile, Raw: path})

	if err != nil || got.V != "hello" {
		t.Fatalf("file value = %#v, %v, want hello without its newline", got.V, err)
	}
	if _, err := r.resolve(arg{Name: "body", Kind: kindFile, Raw: filepath.Join(dir, "missing")}); err == nil {
		t.Fatal("a missing file gave a value")
	}
}

// Scenario: standard input is read once, whole, and a second use is refused
func TestResolve_StdinOnce(t *testing.T) {
	r := resolverWith("s3cret\r\n", nil)

	first, err := r.resolve(arg{Name: "credential", Kind: kindStdin})
	if err != nil || first.V != "s3cret" || !first.Secret {
		t.Fatalf("stdin value = %#v, %v, want s3cret (no newline) marked secret", first.V, err)
	}
	if _, err := r.resolve(arg{Name: "other", Kind: kindStdin}); err == nil {
		t.Fatal("stdin was read twice, so the second field would silently get nothing")
	}
}

// Scenario: which values count as secret
func TestResolve_SecretMarking(t *testing.T) {
	r := resolverWith("x", map[string]string{"E": "e"})
	cases := []struct {
		a      arg
		secret bool
	}{
		{arg{Name: "email", Kind: kindText, Raw: "a@b.test"}, false},
		{arg{Name: "credential", Kind: kindText, Raw: "typed"}, true}, // by its name
		{arg{Name: "userPassword", Kind: kindText, Raw: "typed"}, true},
		{arg{Name: "auth.token", Kind: kindText, Raw: "typed"}, true},
		{arg{Name: "agentKey", Kind: kindText, Raw: "agt_x"}, false}, // the agent key is public
		{arg{Name: "email", Kind: kindEnv, Raw: "E"}, true},          // by its source
	}

	for _, c := range cases {
		got, err := r.resolve(c.a)
		if err != nil {
			t.Fatalf("%+v: %v", c.a, err)
		}
		if got.Secret != c.secret {
			t.Errorf("%s from %v: secret = %v, want %v", c.a.Name, c.a.Kind, got.Secret, c.secret)
		}
	}
}

func TestIsSecretName(t *testing.T) {
	for name, want := range map[string]bool{
		"credential": true, "Credential": true, "access_token": true, "TOKEN": true, "password": true,
		"db.password": true, "client-secret": true, "email": false, "code": false, "agentKey": false, "note": false,
	} {
		if got := isSecretName(name); got != want {
			t.Errorf("isSecretName(%q) = %v, want %v", name, got, want)
		}
	}
}
