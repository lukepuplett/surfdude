package surf

import (
	"errors"
	"strings"
	"testing"

	"github.com/lukepuplett/surfdude/go/hypertext"
)

// Fixed and editable values (docs/model.md): a readOnly input keeps the value the API gave it, a
// caller may not change it, and surf.Unchecked lets a test send what the form forbids.

// fixedControl is a verify-email form: the register response decided the email, so it is prefilled
// and read-only, and the code is the caller's to give.
func fixedControl() hypertext.Control {
	return hypertext.Control{
		Rel:  "verify-email",
		Form: true,
		Inputs: []hypertext.Input{
			{Name: "email", Required: true, Value: "sally@seller.example", Schema: hypertext.Schema{"type": "string", "readOnly": true}},
			{Name: "code", Required: true, Schema: hypertext.Schema{"type": "string", "pattern": "^[0-9]{6}$"}},
		},
	}
}

// Scenario: a read-only input keeps its prefilled value when the caller says nothing about it
func TestMerge_ReadOnlyKeepsItsPrefilledValue(t *testing.T) {
	out, err := merge(fixedControl(), Values{"code": "123456"})

	if err != nil {
		t.Fatal(err)
	}
	if out["email"] != "sally@seller.example" {
		t.Fatalf("email = %v, want the value the form fixed", out["email"])
	}
}

// Scenario: giving a read-only input the value it already has is allowed
func TestMerge_ReadOnlyAcceptsTheSameValue(t *testing.T) {
	out, err := merge(fixedControl(), Values{"code": "123456", "email": "sally@seller.example"})

	if err != nil {
		t.Fatalf("the form's own value was refused: %v", err)
	}
	if out["email"] != "sally@seller.example" {
		t.Fatalf("email = %v", out["email"])
	}
}

// Scenario: a different value for a read-only input is refused, naming the input and no value
func TestMerge_ReadOnlyRefusesADifferentValue(t *testing.T) {
	_, err := merge(fixedControl(), Values{"code": "123456", "email": "other@elsewhere.example"})

	var fixed *hypertext.ReadOnlyInputError
	if !errors.As(err, &fixed) {
		t.Fatalf("err = %v, want *hypertext.ReadOnlyInputError", err)
	}
	if fixed.Name != "email" || fixed.Rel != "verify-email" {
		t.Fatalf("error = %+v, want it to name the input and the rel", fixed)
	}
	for _, leaked := range []string{"other@elsewhere.example", "sally@seller.example"} {
		if strings.Contains(err.Error(), leaked) {
			t.Fatalf("the error repeats %q: a fixed value may be a secret, and errors reach logs", leaked)
		}
	}
}

// Scenario: a read-only input with no value to keep refuses any value, because there is nothing to compare
func TestMerge_ReadOnlyWithoutAPrefilledValueRefusesAnyValue(t *testing.T) {
	c := hypertext.Control{Rel: "r", Inputs: []hypertext.Input{{Name: "token", Schema: hypertext.Schema{"readOnly": true}}}}

	_, err := merge(c, Values{"token": "anything"})

	var fixed *hypertext.ReadOnlyInputError
	if !errors.As(err, &fixed) {
		t.Fatalf("err = %v, want *hypertext.ReadOnlyInputError", err)
	}
}

// Scenario: an input that is not read-only is still free to change, as before
func TestMerge_OrdinaryPrefilledInputCanStillBeChanged(t *testing.T) {
	c := hypertext.Control{Rel: "r", Inputs: []hypertext.Input{{Name: "note", Value: "suggested", Schema: hypertext.Schema{"type": "string"}}}}

	out, err := merge(c, Values{"note": "mine"})

	if err != nil || out["note"] != "mine" {
		t.Fatalf("out = %v, err = %v, want the caller's value", out, err)
	}
}

// Scenario: unchecked sends what the form forbids: an unknown field, a missing required one, a value
// outside its schema, and a different value for a read-only input
func TestMergeUnchecked_SendsWhatTheFormForbids(t *testing.T) {
	out, err := mergeWith(fixedControl(), Values{"code": "not six digits", "email": "other@elsewhere.example", "bogus": "1"}, true)

	if err != nil {
		t.Fatalf("an unchecked step was refused: %v", err)
	}
	if out["code"] != "not six digits" || out["bogus"] != "1" || out["email"] != "other@elsewhere.example" {
		t.Fatalf("out = %v, want every value exactly as given", out)
	}

	out, err = mergeWith(fixedControl(), Values{}, true)
	if err != nil {
		t.Fatalf("an unchecked step with a required field missing was refused: %v", err)
	}
	if _, has := out["code"]; has {
		t.Fatalf("out = %v, want the missing code to stay missing", out)
	}
}

// Scenario: unchecked still sends the values the form prefilled
func TestMergeUnchecked_StillSendsPrefilledValues(t *testing.T) {
	out, err := mergeWith(fixedControl(), Values{"bogus": "1"}, true)

	if err != nil {
		t.Fatal(err)
	}
	if out["email"] != "sally@seller.example" {
		t.Fatalf("email = %v, want the prefilled value even when unchecked", out["email"])
	}
}

// Scenario: checked is still the default, so the three refusals stand
func TestMerge_CheckedStillRefusesUnknownMissingAndInvalid(t *testing.T) {
	for name, v := range map[string]Values{
		"an unknown field":       {"code": "123456", "bogus": "1"},
		"a required one missing": {},
		"a value outside schema": {"code": "abc"},
	} {
		if _, err := merge(fixedControl(), v); err == nil {
			t.Errorf("%s was accepted by a checked merge", name)
		}
	}
}
