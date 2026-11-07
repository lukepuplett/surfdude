package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lukepuplett/surfdude/go/hypertext"
)

// Feature: surf describe ROOT REL (what does this form take?)

// Scenario: describe lists the fields, what is required and the limits, and a line to start from
func TestDescribe_ListsFieldsLimitsAndATemplate(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, "describe", api.URL, "set-count")

	if r.Code != ExitOK {
		t.Fatalf("exit %d\nstderr: %s", r.Code, r.ErrOut)
	}
	for _, want := range []string{
		"set-count", "POST", api.URL + "/count",
		"count*", "integer", "min 1", "max 10", "how many",
		"note", "string", "max 20 chars",
		"surf " + api.URL + " set-count count:=VALUE [note=TEXT]",
	} {
		if !strings.Contains(r.Out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.Out)
		}
	}
	if len(api.all()) != 1 {
		t.Fatalf("%d requests were sent, want only the read of the front door", len(api.all()))
	}
}

// Scenario: pattern and format limits are shown, and a link's address template is a field too
func TestDescribe_ShowsPatternsFormatsAndAddressTemplates(t *testing.T) {
	api := newFakeAPI(t)

	verify := runCLI(t, "", nil, "describe", api.URL, "verify-email")
	for _, want := range []string{"email*", "format email", "code*", "pattern ^[0-9]{6}$"} {
		if !strings.Contains(verify.Out, want) {
			t.Errorf("verify-email: stdout lacks %q:\n%s", want, verify.Out)
		}
	}

	outbox := runCLI(t, "", nil, "describe", api.URL, "outbox")
	if !strings.Contains(outbox.Out, "to") || !strings.Contains(outbox.Out, "GET") || !strings.Contains(outbox.Out, "surf "+api.URL+" outbox [to=TEXT]") {
		t.Errorf("outbox: the address template is not shown as a field:\n%s", outbox.Out)
	}
}

// Scenario: a rel the server does not offer is exit 5, and the message lists what it does offer
func TestDescribe_UnknownRelListsWhatIsOffered(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, "describe", api.URL, "nope")

	if r.Code != ExitNotOffered || !strings.Contains(r.ErrOut, "register") || !strings.Contains(r.ErrOut, "set-count") {
		t.Fatalf("exit %d, stderr %q, want 5 and the offered rels", r.Code, r.ErrOut)
	}
}

func TestDescribe_NeedsARootAndARel(t *testing.T) {
	if r := runCLI(t, "", nil, "describe", "http://x.test"); r.Code != ExitUsage {
		t.Fatalf("exit %d, want 2", r.Code)
	}
}

// Scenario: a prefilled value is shown, unless its name is on the hidden list
func TestDescribeControl_PrefilledValues(t *testing.T) {
	c := hypertext.Control{
		Rel: "set", Href: "/x", Method: "POST", Form: true,
		Inputs: []hypertext.Input{
			{Name: "colour", Value: "red", Schema: hypertext.Schema{"type": "string", "enum": []any{"red", "blue"}}},
			{Name: "accessToken", Value: "tok-prefilled-99", Schema: hypertext.Schema{"type": "string"}},
		},
	}
	var out bytes.Buffer

	describeControl(&out, "http://x.test", c, masking{})

	got := out.String()
	if !strings.Contains(got, "prefilled: red") || !strings.Contains(got, "one of red, blue") {
		t.Errorf("the prefilled value or the enum is missing:\n%s", got)
	}
	if strings.Contains(got, "tok-prefilled-99") || !strings.Contains(got, "prefilled: <secret>") {
		t.Errorf("a secret-named prefilled value was shown:\n%s", got)
	}
}
