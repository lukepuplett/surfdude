package surf

import (
	"testing"

	"github.com/lukepuplett/surfdude/go/hypertext"
)

func TestMerge_PrefilledInputIsSentUnlessOverridden(t *testing.T) {
	c := hypertext.Control{
		Rel:  "save",
		Form: true,
		Inputs: []hypertext.Input{
			{Name: "sessionToken", Value: "tok_abc"},
			{Name: "title", Required: true},
		},
	}

	out, err := merge(c, Values{"title": "Hello"})
	if err != nil {
		t.Fatal(err)
	}
	if out["sessionToken"] != "tok_abc" || out["title"] != "Hello" {
		t.Fatalf("merge = %v, want prefilled token and caller title", out)
	}

	out, err = merge(c, Values{"sessionToken": "override", "title": "Hello"})
	if err != nil {
		t.Fatal(err)
	}
	if out["sessionToken"] != "override" {
		t.Fatalf("caller should override prefill, got %v", out["sessionToken"])
	}
}

func TestMerge_PrefillSatisfiesRequiredWithoutCallerValue(t *testing.T) {
	c := hypertext.Control{
		Rel: "shortlist",
		Inputs: []hypertext.Input{
			{Name: "listingId", Required: true, Value: "lst_1"},
			{Name: "note"},
		},
	}

	out, err := merge(c, Values{"note": "nice"})
	if err != nil {
		t.Fatal(err)
	}
	if out["listingId"] != "lst_1" {
		t.Fatalf("listingId = %v, want prefilled lst_1", out["listingId"])
	}
}
