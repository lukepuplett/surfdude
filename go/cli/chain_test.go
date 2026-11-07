package cli

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// Feature: a chain of steps, joined by the word then

func TestParseSteps_OneStep(t *testing.T) {
	steps, err := parseSteps([]string{"register", "email=me@example.test", "count:=3"})

	if err != nil || len(steps) != 1 {
		t.Fatalf("steps = %+v, err = %v, want one step", steps, err)
	}
	s := steps[0]
	if s.Rel != "register" || len(s.Args) != 2 || s.Args[0].Name != "email" || s.Args[1].Kind != kindJSON {
		t.Fatalf("step = %+v", s)
	}
}

func TestParseSteps_ThenSplitsTheChain(t *testing.T) {
	steps, err := parseSteps(strings.Fields("register email=a@b.test then verify-email code={code} then sign-in credential=env:C"))

	if err != nil || len(steps) != 3 {
		t.Fatalf("steps = %d, err = %v, want 3", len(steps), err)
	}
	var rels []string
	for _, s := range steps {
		rels = append(rels, s.Rel)
	}
	if !reflect.DeepEqual(rels, []string{"register", "verify-email", "sign-in"}) {
		t.Fatalf("rels = %v", rels)
	}
}

func TestParseSteps_KeepAndSecretBelongToTheirStep(t *testing.T) {
	steps, err := parseSteps(strings.Fields("register email=a@b.test --keep code=data.0.text --secret password then sign-in"))

	if err != nil || len(steps) != 2 {
		t.Fatalf("steps = %d, err = %v, want 2", len(steps), err)
	}
	if !reflect.DeepEqual(steps[0].Keeps, []keep{{Name: "code", Path: "data.0.text"}}) {
		t.Errorf("keeps = %+v", steps[0].Keeps)
	}
	if !reflect.DeepEqual(steps[0].Secrets, []string{"password"}) {
		t.Errorf("secrets = %+v", steps[0].Secrets)
	}
	if len(steps[1].Keeps) != 0 || len(steps[1].Secrets) != 0 {
		t.Errorf("the second step picked up the first step's flags: %+v", steps[1])
	}
}

// Scenario: --keep can name a pattern after ~ to pull part of the text out
func TestParseSteps_KeepWithAPattern(t *testing.T) {
	steps, err := parseSteps([]string{"outbox", "--keep", "code=data.0.text~[0-9]{6}"})

	if err != nil || len(steps) != 1 || len(steps[0].Keeps) != 1 {
		t.Fatalf("steps = %+v, err = %v", steps, err)
	}
	if k := steps[0].Keeps[0]; k.Name != "code" || k.Path != "data.0.text" || k.Pattern != "[0-9]{6}" {
		t.Fatalf("keep = %+v, want path data.0.text and pattern [0-9]{6}", k)
	}

	_, err = parseSteps([]string{"outbox", "--keep", "code=data.0.text~[unclosed"})
	var u *usageError
	if !errors.As(err, &u) {
		t.Fatalf("an invalid pattern gave %v, want a usage error", err)
	}
}

// Scenario: a chain that cannot be read is a usage error, before anything is sent
func TestParseSteps_Refusals(t *testing.T) {
	for _, line := range []string{
		"",              // nothing to do
		"then register", // starts with then
		"register then", // ends with then
		"register then then sign-in",
		"=x",              // no rel
		"-x",              // a flag where the rel belongs
		"register --keep", // --keep needs name=path
		"register --keep nopath",
		"register --keep code=data.0.text~", // a ~ with no pattern after it
		"register --keep bad_name_only=",
		"register --keep 'bad name=x'",
		"register --secret",
		"register --nope",         // flags go before the root
		"register x=1 x=2",        // the same field twice
		"register x=1 --secret x", // a field given and also prompted
	} {
		_, err := parseSteps(strings.Fields(line))
		var u *usageError
		if !errors.As(err, &u) {
			t.Errorf("parseSteps(%q) err = %v, want a usage error", line, err)
		}
	}
}

// Scenario: a {name} must be kept by an earlier step, which is checked before anything is sent
func TestValidateTemplates(t *testing.T) {
	ok := "register --keep code=data.0.text then verify-email code={code}"
	steps, _ := parseSteps(strings.Fields(ok))
	if err := validateTemplates(steps); err != nil {
		t.Fatalf("a chain that keeps what it uses was refused: %v", err)
	}

	for name, line := range map[string]string{
		"never kept":        "register then verify-email code={code}",
		"kept by a later":   "register code={code} then verify-email --keep code=data.0.text",
		"kept by this step": "register --keep code=data.0.text code={code}",
	} {
		steps, err := parseSteps(strings.Fields(line))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		err = validateTemplates(steps)
		var u *usageError
		if !errors.As(err, &u) || !strings.Contains(err.Error(), "{code}") {
			t.Errorf("%s: err = %v, want a usage error that names {code}", name, err)
		}
	}
}

// Scenario: only name=text is a template: JSON, files, stdin and the environment are sent as they are
func TestValidateTemplates_OnlyTextValuesAreTemplates(t *testing.T) {
	steps, err := parseSteps([]string{"register", `note:="{code}"`, "a=env:X", "b=@file{code}", "c=plain"})
	if err != nil {
		t.Fatal(err)
	}

	if err := validateTemplates(steps); err != nil {
		t.Fatalf("a {code} in a JSON, file or environment value was treated as a template: %v", err)
	}
}

func TestExpandTemplate(t *testing.T) {
	kept := map[string]keptValue{"code": {Text: "123456"}, "credential": {Text: "agt_x", Secret: true}}

	got, secret, err := expandTemplate("code is {code}!", kept)
	if err != nil || got != "code is 123456!" || secret {
		t.Fatalf("got %q, secret %v, err %v", got, secret, err)
	}
	got, secret, err = expandTemplate("Bearer {credential}", kept)
	if err != nil || got != "Bearer agt_x" || !secret {
		t.Fatalf("got %q, secret %v, err %v, want the result marked secret because a secret went in", got, secret, err)
	}
	got, _, _ = expandTemplate(`{"a":1} and {not a name}`, kept)
	if got != `{"a":1} and {not a name}` {
		t.Fatalf("braces that are not a name were changed: %q", got)
	}
	if _, _, err := expandTemplate("{missing}", kept); err == nil {
		t.Fatal("an unknown {name} was left in place instead of refused")
	}
}
