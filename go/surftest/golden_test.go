package surftest_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lukepuplett/surfdude/go/dialects/agenttrader"
	"github.com/lukepuplett/surfdude/go/surf"
	"github.com/lukepuplett/surfdude/go/surftest"
)

const updateEnv = "SURFDUDE_UPDATE_GOLDEN"

// recorder is a testing.TB that records a failure instead of failing the real test, so a test can
// check that Golden fails when it should and what it says.
type recorder struct {
	testing.TB
	failed   bool
	messages []string
}

type stopped struct{}

func (r *recorder) Errorf(format string, args ...any) {
	r.failed = true
	r.messages = append(r.messages, fmt.Sprintf(format, args...))
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.Errorf(format, args...)
	panic(stopped{})
}

func (r *recorder) Fatal(args ...any) { r.Fatalf("%s", fmt.Sprint(args...)) }

// expectFailure runs f against a recorder and returns what it reported.
func expectFailure(t *testing.T, f func(tb testing.TB)) *recorder {
	t.Helper()
	r := &recorder{TB: t}
	func() {
		defer func() {
			if v := recover(); v != nil {
				if _, ok := v.(stopped); !ok {
					panic(v)
				}
			}
		}()
		f(r)
	}()
	return r
}

// api serves one envelope that carries the server's own address (as the home page's owner prompts do)
// and an expiry that changes on every request, which is what a recorded sample has to cope with.
func api(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Add("Link", `<https://agenttrader.net/profiles/envelope/v1>; rel="profile"`)
		fmt.Fprintf(w, `{"status":200,"message":"OK","notes":["Start at %s/api","See https://example.com/docs"],`+
			`"data":[{"name":"Fake","expiresAt":"2026-11-02T12:%02d:00Z","price":3000}],`+
			`"_links":[{"rel":"self","href":"/","method":"GET"}]}`, srv.URL, n)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func page(t *testing.T, srv *httptest.Server) *surf.Page {
	t.Helper()
	s := surftest.New(t, srv.URL+"/", surf.WithDialects(agenttrader.New()))
	p, err := s.Open(context.Background(), "/")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Scenario: a sample is recorded from the real response, then checked
//
//	Given a server whose response mentions its own address
//	When the golden helper runs in update mode
//	Then the file holds the response with the address replaced by {base}, indented, in the server's key order
//	When it runs again without update mode
//	Then it passes
func TestGolden_RecordsThenChecks(t *testing.T) {
	// Arrange
	srv, _ := api(t)
	file := filepath.Join(t.TempDir(), "samples", "home.json")

	// Act: record
	t.Setenv(updateEnv, "1")
	surftest.Golden(t, page(t, srv), file, surftest.Mask("data[0].expiresAt"))

	// Assert: what was written
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("the golden file was not written: %v", err)
	}
	text := string(raw)
	if strings.Contains(text, srv.URL) {
		t.Errorf("the file still contains the test server's address %s:\n%s", srv.URL, text)
	}
	if !strings.Contains(text, `"Start at {base}/api"`) {
		t.Errorf("the address should become {base}, got:\n%s", text)
	}
	if !strings.Contains(text, "https://example.com/docs") {
		t.Errorf("another host must be left alone, got:\n%s", text)
	}
	if !(strings.Index(text, `"status"`) < strings.Index(text, `"notes"`) && strings.Index(text, `"notes"`) < strings.Index(text, `"data"`)) {
		t.Errorf("keys should stay in the order the server sent them, got:\n%s", text)
	}
	if !strings.HasPrefix(text, "{\n  \"status\": 200,") || !strings.HasSuffix(text, "}\n") {
		t.Errorf("the file should be indented by two spaces and end with a newline, got:\n%s", text)
	}

	// Act and assert: check mode, with the volatile expiry masked, passes on a later response
	t.Setenv(updateEnv, "")
	surftest.Golden(t, page(t, srv), file, surftest.Mask("data[0].expiresAt"))
}

func TestGolden_MaskedFieldsAreWrittenAsPlaceholders(t *testing.T) {
	srv, _ := api(t)
	file := filepath.Join(t.TempDir(), "home.json")
	t.Setenv(updateEnv, "1")

	surftest.Golden(t, page(t, srv), file, surftest.Mask("data[0].expiresAt", "no.such.path"))

	raw, _ := os.ReadFile(file)
	if !strings.Contains(string(raw), `"expiresAt": "<masked>"`) {
		t.Errorf("the masked field should read <masked>, got:\n%s", raw)
	}
	if !strings.Contains(string(raw), `"price": 3000`) {
		t.Errorf("a field that is not masked must keep its value, got:\n%s", raw)
	}
}

// Scenario: a changed response fails the check and says where
func TestGolden_WhenTheResponseChanged_FailsNamingTheDifferences(t *testing.T) {
	// Arrange: record, then change the file the way a code change would
	srv, _ := api(t)
	file := filepath.Join(t.TempDir(), "home.json")
	t.Setenv(updateEnv, "1")
	surftest.Golden(t, page(t, srv), file, surftest.Mask("data[0].expiresAt"))
	t.Setenv(updateEnv, "")
	raw, _ := os.ReadFile(file)
	edited := strings.Replace(string(raw), `"price": 3000`, `"price": 2500`, 1)
	if err := os.WriteFile(file, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	// Act
	r := expectFailure(t, func(tb testing.TB) {
		surftest.Golden(tb, page(t, srv), file, surftest.Mask("data[0].expiresAt"))
	})

	// Assert
	if !r.failed {
		t.Fatal("a response that differs from the golden file must fail")
	}
	msg := strings.Join(r.messages, "\n")
	for _, want := range []string{"data[0].price", "2500", "3000", updateEnv} {
		if !strings.Contains(msg, want) {
			t.Errorf("the failure should mention %q so the change can be found and accepted, got:\n%s", want, msg)
		}
	}
}

func TestGolden_WithoutMasking_AVolatileFieldFails(t *testing.T) {
	srv, _ := api(t)
	file := filepath.Join(t.TempDir(), "home.json")
	t.Setenv(updateEnv, "1")
	surftest.Golden(t, page(t, srv), file)
	t.Setenv(updateEnv, "")

	r := expectFailure(t, func(tb testing.TB) { surftest.Golden(tb, page(t, srv), file) })

	if !r.failed || !strings.Contains(strings.Join(r.messages, "\n"), "data[0].expiresAt") {
		t.Errorf("an unmasked changing field should fail naming data[0].expiresAt, got failed=%v %v", r.failed, r.messages)
	}
}

func TestGolden_WhenThereIsNoFile_SaysHowToCreateIt(t *testing.T) {
	srv, _ := api(t)
	file := filepath.Join(t.TempDir(), "missing.json")
	t.Setenv(updateEnv, "")

	r := expectFailure(t, func(tb testing.TB) { surftest.Golden(tb, page(t, srv), file) })

	if !r.failed || !strings.Contains(strings.Join(r.messages, "\n"), updateEnv) {
		t.Errorf("a missing file should fail and mention %s, got failed=%v %v", updateEnv, r.failed, r.messages)
	}
	if _, err := os.Stat(file); err == nil {
		t.Error("check mode must never write the file")
	}
}

func TestGolden_WhenTheBodyIsNotJSON_Fails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "nothing here")
	}))
	t.Cleanup(srv.Close)
	s := surftest.New(t, srv.URL+"/", surf.WithDialects(agenttrader.New()))
	p, _ := s.Open(context.Background(), "/") // an unreadable 404 still gives a page with the body (GAP-11)
	t.Setenv(updateEnv, "1")

	r := expectFailure(t, func(tb testing.TB) { surftest.Golden(tb, p, filepath.Join(t.TempDir(), "x.json")) })

	if !r.failed || !strings.Contains(strings.Join(r.messages, "\n"), "JSON") {
		t.Errorf("a body that is not JSON should fail saying so, got failed=%v %v", r.failed, r.messages)
	}
}
