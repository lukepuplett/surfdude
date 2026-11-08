package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/lukepuplett/surfdude/go/hypertext"
)

// Feature: fixed and editable values (docs/model.md)
//
// A field says "change this if you like". A readOnly field says "this is decided": it is sent with
// the value the API gave it, and the CLI refuses a different one. A value fixed in the href that a
// field repeats is a mistake in the API, and describe and explore say so.

// formAPI is an API whose front door offers the actions it is given. A POST to /start answers with
// the actions it is given as startOffers, and any other POST is recorded and answers 200.
type formAPI struct {
	*httptest.Server
	mu    sync.Mutex
	posts []string
}

func newFormAPI(t *testing.T, actions, startOffers string) *formAPI {
	t.Helper()
	f := &formAPI{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Add("Link", profileLink)
		switch {
		case r.Method == http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			f.mu.Lock()
			f.posts = append(f.posts, string(body))
			f.mu.Unlock()
			if r.URL.Path == "/start" {
				w.WriteHeader(http.StatusAccepted)
				_, _ = io.WriteString(w, `{"status":202,"message":"Accepted","data":[],"_actions":[`+startOffers+`]}`)
				return
			}
			_, _ = io.WriteString(w, `{"status":200,"message":"OK","data":[]}`)
		case r.URL.Path == "/":
			_, _ = io.WriteString(w, `{"status":200,"message":"OK","data":[],"_actions":[`+actions+`]}`)
		default:
			_, _ = io.WriteString(w, `{"status":200,"message":"OK","data":[]}`)
		}
	}))
	t.Cleanup(f.Close)

	return f
}

func (f *formAPI) sent() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.posts...)
}

const (
	// verifyFixed is verify-email as the API should offer it: the email was decided by the register
	// response, so it is prefilled and read-only, and the code is the caller's to give.
	verifyFixed = `{"rel":"verify-email","action":"/verify","method":"POST","fields":[` +
		`{"name":"email","required":true,"value":"fixed@example.test","schema":{"type":"string","readOnly":true}},` +
		`{"name":"code","required":true,"schema":{"type":"string"}}]}`
	startForm = `{"rel":"register","action":"/start","method":"POST","fields":[{"name":"email","required":true,"schema":{"type":"string"}}]}`
	// mailboxRepeats fixes `to` in the href and also declares it as a field.
	mailboxRepeats = `{"rel":"mailbox","action":"/box?to=a%40b.test","method":"GET","fields":[{"name":"to"},{"name":"from"}]}`
)

// Scenario: a read-only input is sent with the value the form gave it, and the caller need not give it
func TestReadOnly_TheFormsValueIsSentWithoutTheCallerGivingIt(t *testing.T) {
	api := newFormAPI(t, verifyFixed, "")

	r := runCLI(t, "", nil, api.URL, "verify-email", "code=123456")

	if r.Code != ExitOK {
		t.Fatalf("exit %d, want 0\nstderr: %s", r.Code, r.ErrOut)
	}
	if got := api.sent(); len(got) != 1 || got[0] != `{"code":"123456","email":"fixed@example.test"}` {
		t.Fatalf("sent %q, want the form's email and the caller's code", got)
	}
}

// Scenario: giving a read-only input the value it already has is allowed
func TestReadOnly_TheSameValueIsAccepted(t *testing.T) {
	api := newFormAPI(t, verifyFixed, "")

	r := runCLI(t, "", nil, api.URL, "verify-email", "code=123456", "email=fixed@example.test")

	if r.Code != ExitOK {
		t.Fatalf("exit %d, want 0\nstderr: %s", r.Code, r.ErrOut)
	}
}

// Scenario: a refused read-only input is a usage error wherever the CLI rejects it, not a network failure
func TestReadOnly_IsAUsageErrorForTheExitCode(t *testing.T) {
	if got := exitFor(&hypertext.ReadOnlyInputError{Rel: "verify-email", Name: "email"}); got != ExitUsage {
		t.Fatalf("exitFor = %d, want %d (usage)", got, ExitUsage)
	}
}

// Scenario: a different value for a read-only input is a usage error, sends nothing, and prints no value
func TestReadOnly_ADifferentValueIsRefusedAndNoValueIsPrinted(t *testing.T) {
	api := newFormAPI(t, verifyFixed, "")

	r := runCLI(t, "", nil, api.URL, "verify-email", "code=123456", "email=other@example.test")

	if r.Code != ExitUsage {
		t.Fatalf("exit %d, want %d\nstderr: %s", r.Code, ExitUsage, r.ErrOut)
	}
	if got := api.sent(); len(got) != 0 {
		t.Fatalf("sent %q, want nothing", got)
	}
	if !strings.Contains(r.ErrOut, "email") || !strings.Contains(r.ErrOut, "fixed") {
		t.Errorf("stderr should name the fixed input:\n%s", r.ErrOut)
	}
	for _, leaked := range []string{"other@example.test", "fixed@example.test"} {
		if strings.Contains(r.ErrOut+r.Out, leaked) {
			t.Errorf("a value was printed: %q", leaked)
		}
	}
}

// Scenario: --unchecked overrides a read-only input, as it overrides every other check
func TestReadOnly_UncheckedOverridesIt(t *testing.T) {
	api := newFormAPI(t, verifyFixed, "")

	r := runCLI(t, "", nil, api.URL, "verify-email", "code=123456", "email=other@example.test", "--unchecked")

	if r.Code != ExitOK {
		t.Fatalf("exit %d, want 0\nstderr: %s", r.Code, r.ErrOut)
	}
	if got := api.sent(); len(got) != 1 || !strings.Contains(got[0], `"email":"other@example.test"`) {
		t.Fatalf("sent %q, want the overriding email", got)
	}
}

// Scenario: a read-only input with no value to keep refuses any value
func TestReadOnly_WithNoValueToKeepRefusesAnyValue(t *testing.T) {
	api := newFormAPI(t, `{"rel":"set","action":"/set","method":"POST","fields":[{"name":"token_id","schema":{"readOnly":true}}]}`, "")

	r := runCLI(t, "", nil, api.URL, "set", "token_id=anything")

	if r.Code != ExitUsage || len(api.sent()) != 0 {
		t.Fatalf("exit %d, sent %q, want a usage error and nothing sent", r.Code, api.sent())
	}
}

// Scenario: describe marks a read-only field as fixed by the API, and does not ask for it
func TestReadOnly_DescribeMarksItFixedAndDoesNotAskForIt(t *testing.T) {
	api := newFormAPI(t, verifyFixed, "")

	r := runCLI(t, "", nil, "describe", api.URL, "verify-email")

	if r.Code != ExitOK {
		t.Fatalf("exit %d\nstderr: %s", r.Code, r.ErrOut)
	}
	var emailLine, tryLine string
	lines := strings.Split(r.Out, "\n")
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "email") {
			emailLine = l
		}
		if strings.HasPrefix(l, "try it:") && i+1 < len(lines) {
			tryLine = lines[i+1]
		}
	}
	if !strings.Contains(emailLine, "fixed by the API") || !strings.Contains(emailLine, "fixed@example.test") {
		t.Errorf("the email line should say it is fixed, and by what value: %q", emailLine)
	}
	if strings.Contains(tryLine, "email=") {
		t.Errorf("the try-it line asks for a fixed field: %q", tryLine)
	}
	if !strings.Contains(tryLine, "code=TEXT") {
		t.Errorf("the try-it line should still ask for the code: %q", tryLine)
	}
}

// Scenario: a fixed value under a hidden name is hidden in describe, and shown with --reveal
func TestReadOnly_DescribeHidesAFixedSecretUnlessRevealed(t *testing.T) {
	api := newFormAPI(t, `{"rel":"use","action":"/use","method":"POST","fields":[`+
		`{"name":"credential","value":"s3cr3t-fixed-77","schema":{"type":"string","readOnly":true}}]}`, "")

	hidden := runCLI(t, "", nil, "describe", api.URL, "use")
	revealed := runCLI(t, "", nil, "--reveal", "describe", api.URL, "use")

	if strings.Contains(hidden.Out, "s3cr3t-fixed-77") || !strings.Contains(hidden.Out, secretMask) {
		t.Errorf("a fixed credential should be hidden:\n%s", hidden.Out)
	}
	if !strings.Contains(hidden.Out, "fixed by the API") {
		t.Errorf("it should still say the field is fixed:\n%s", hidden.Out)
	}
	if !strings.Contains(revealed.Out, "s3cr3t-fixed-77") {
		t.Errorf("--reveal should show it:\n%s", revealed.Out)
	}
}

// Scenario: the next list marks a read-only field, compactly
func TestReadOnly_TheNextListMarksIt(t *testing.T) {
	api := newFormAPI(t, startForm, verifyFixed)

	r := runCLI(t, "", nil, api.URL, "register", "email=a@example.test")

	if r.Code != ExitOK {
		t.Fatalf("exit %d\nstderr: %s", r.Code, r.ErrOut)
	}
	if !strings.Contains(r.Out, "email*[fixed] code*") {
		t.Fatalf("the next list should mark the fixed field:\n%s", r.Out)
	}
}

// Scenario: describe warns, on stderr, about a field that repeats a fixed parameter of the href
func TestCollision_DescribeWarns(t *testing.T) {
	api := newFormAPI(t, mailboxRepeats, "")

	r := runCLI(t, "", nil, "describe", api.URL, "mailbox")

	if r.Code != ExitOK {
		t.Fatalf("exit %d, want 0: a warning does not fail describe\nstderr: %s", r.Code, r.ErrOut)
	}
	if !strings.Contains(r.ErrOut, `warning: field "to" repeats a parameter fixed in the href.`) {
		t.Errorf("stderr lacks the warning:\n%s", r.ErrOut)
	}
	if strings.Contains(r.ErrOut, `"from"`) {
		t.Errorf("a field that repeats nothing was warned about:\n%s", r.ErrOut)
	}
	if !strings.Contains(r.Out, "to") || !strings.Contains(r.Out, "from") {
		t.Errorf("describe should still list the fields:\n%s", r.Out)
	}
}

// Scenario: describe says nothing when no field repeats a fixed parameter
func TestCollision_DescribeIsQuietWhenNothingRepeats(t *testing.T) {
	api := newFormAPI(t, `{"rel":"mailbox","action":"/box?to=a%40b.test","method":"GET","fields":[{"name":"from"}]}`, "")

	r := runCLI(t, "", nil, "describe", api.URL, "mailbox")

	if strings.Contains(r.ErrOut, "warning") {
		t.Fatalf("unexpected warning:\n%s", r.ErrOut)
	}
}

// Scenario: explore reports it as a problem, so the run exits 1
func TestCollision_ExploreCountsItAsAProblem(t *testing.T) {
	api := newFormAPI(t, mailboxRepeats, "")

	r := runCLI(t, "", nil, "explore", api.URL)

	if r.Code != 1 {
		t.Fatalf("exit %d, want 1\nstdout: %s\nstderr: %s", r.Code, r.Out, r.ErrOut)
	}
	if !strings.Contains(r.Out, `field "to" repeats a parameter fixed in the href`) {
		t.Errorf("explore should say which field:\n%s", r.Out)
	}
}
