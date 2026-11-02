package surf_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/lukepuplett/surfdude/go/dialects/agenttrader"
	"github.com/lukepuplett/surfdude/go/hypertext"
	"github.com/lukepuplett/surfdude/go/surf"
)

// authServer is a small API with a session: sign in with the credential "good" to get a token, and
// protected routes answer 401 (with the sign-in action in the body) to any other token.
type authServer struct {
	*httptest.Server
	mu        sync.Mutex
	valid     map[string]bool
	issued    int
	signIns   int
	seenAuth  []string // the Authorization header of each protected request, in order
	offerBody []string // the body of each offer request, in order
	refuseNew bool     // issue tokens that the protected routes then refuse
}

const (
	signInAction = `{"rel":"sign-in","action":"/auth/token","method":"POST","enctype":"application/json","fields":[{"name":"credential","required":true,"schema":{"type":"string"}}]}`
	offerAction  = `{"rel":"make-offer","action":"/offers","method":"POST","enctype":"application/json","requires":["agent-session"],"fields":[{"name":"amount","required":true,"schema":{"type":"integer"}}]}`
)

func newAuthServer(t *testing.T) *authServer {
	t.Helper()
	a := &authServer{valid: map[string]bool{}}
	mux := http.NewServeMux()

	envelope := func(w http.ResponseWriter, status int, body string) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Add("Link", profile)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
	protected := func(w http.ResponseWriter, r *http.Request, record func(string)) bool {
		a.mu.Lock()
		defer a.mu.Unlock()
		auth := r.Header.Get("Authorization")
		a.seenAuth = append(a.seenAuth, auth)
		if a.valid[strings.TrimPrefix(auth, "Bearer ")] {
			return true
		}
		w.Header().Set("WWW-Authenticate", `Bearer realm="test", error="invalid_token"`)
		envelope(w, 401, `{"status":401,"message":"Sign in again","notes":["missing: agent-session"],"data":[],"_actions":[`+signInAction+`]}`)
		return false
	}

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		envelope(w, 200, `{"status":200,"message":"OK","data":[{"name":"Fake"}],`+
			`"_links":[{"rel":"protected","href":"/protected","method":"GET","requires":["agent-session"]}],`+
			`"_actions":[`+signInAction+`,`+offerAction+`]}`)
	})
	mux.HandleFunc("POST /auth/token", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Credential string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Credential != "good" {
			envelope(w, 401, `{"status":401,"message":"Not right","data":[]}`)
			return
		}
		a.mu.Lock()
		a.signIns++
		a.issued++
		token := fmt.Sprintf("T%d", a.issued)
		if !a.refuseNew {
			a.valid[token] = true
		}
		a.mu.Unlock()
		envelope(w, 200, fmt.Sprintf(`{"status":200,"message":"OK","data":[{"token":%q}]}`, token))
	})
	mux.HandleFunc("GET /protected", func(w http.ResponseWriter, r *http.Request) {
		if protected(w, r, nil) {
			envelope(w, 200, `{"status":200,"message":"OK","data":[{"secret":"yes"}]}`)
		}
	})
	mux.HandleFunc("POST /offers", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if protected(w, r, nil) {
			a.mu.Lock()
			a.offerBody = append(a.offerBody, string(body))
			a.mu.Unlock()
			envelope(w, 200, `{"status":200,"message":"OK","data":[{"accepted":true}]}`)
		}
	})
	mux.HandleFunc("GET /expire", func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		a.valid = map[string]bool{}
		a.mu.Unlock()
	})

	a.Server = httptest.NewServer(mux)
	t.Cleanup(a.Close)
	return a
}

func (a *authServer) counts() (signIns, protectedRequests int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.signIns, len(a.seenAuth)
}

// signInWith is the kind of hook a test would write: follow the sign-in control on the page that
// showed the need, send the credential, and hand back the token from the response.
func signInWith(credential string, calls *atomic.Int32) surf.SignInFunc {
	return func(ctx context.Context, from *surf.Page) (string, error) {
		calls.Add(1)
		next, err := from.Submit(ctx, "sign-in", surf.Values{"credential": credential})
		if err != nil {
			return "", err
		}
		var got struct {
			Token string `json:"token"`
		}
		if err := next.Read(&got); err != nil {
			return "", err
		}
		return got.Token, nil
	}
}

func authSurfer(t *testing.T, a *authServer, opts ...surf.Option) *surf.Surfer {
	t.Helper()
	s, err := surf.New(a.URL+"/", append([]surf.Option{surf.WithDialects(agenttrader.New())}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Scenario: a refused step signs in again and carries on
//
//	Given a surfer holding a token the server no longer accepts
//	When a journey follows a protected link and the server answers 401
//	Then the surfer follows the sign-in control from the 401 page, keeps the new token
//	And sends the same request again, once, and the journey carries on
func TestWithSignIn_On401_SignsInAndRetriesOnce(t *testing.T) {
	// Arrange
	a := newAuthServer(t)
	var calls atomic.Int32
	s := authSurfer(t, a, surf.WithBearer("stale"), surf.WithSignIn(signInWith("good", &calls)))
	home, err := s.Root(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Act
	page, err := home.Follow(context.Background(), "protected")

	// Assert
	if err != nil {
		t.Fatalf("the step should have signed in and carried on, got %v", err)
	}
	if page.Status != 200 {
		t.Errorf("Status = %d, want 200", page.Status)
	}
	if calls.Load() != 1 {
		t.Errorf("the sign-in hook ran %d times, want once", calls.Load())
	}
	if signIns, requests := a.counts(); signIns != 1 || requests != 2 {
		t.Errorf("server saw %d sign-ins and %d protected requests, want 1 and 2 (stale, then new)", signIns, requests)
	}
	if got := a.seenAuth; len(got) != 2 || got[0] != "Bearer stale" || got[1] != "Bearer T1" {
		t.Errorf("Authorization headers = %v, want [Bearer stale, Bearer T1]", got)
	}
	// and the new token is kept, so the next step needs no sign-in at all
	if _, err := home.Follow(context.Background(), "protected"); err != nil {
		t.Errorf("the next step should reuse the new token, got %v", err)
	}
	if signIns, _ := a.counts(); signIns != 1 {
		t.Errorf("a second step signed in again (%d sign-ins), want the token reused", signIns)
	}
}

// A form's body is sent again intact after signing in.
func TestWithSignIn_ReplaysAFormBodyIntact(t *testing.T) {
	a := newAuthServer(t)
	var calls atomic.Int32
	s := authSurfer(t, a, surf.WithBearer("stale"), surf.WithSignIn(signInWith("good", &calls)))
	home, _ := s.Root(context.Background())

	_, err := home.Submit(context.Background(), "make-offer", surf.Values{"amount": 2750})

	if err != nil {
		t.Fatalf("make-offer should sign in and carry on, got %v", err)
	}
	if len(a.offerBody) != 1 || !strings.Contains(a.offerBody[0], `"amount":2750`) {
		t.Errorf("the server received offer bodies %v, want one with amount 2750 after the replay", a.offerBody)
	}
}

// Scenario: a control that needs a session signs in first when the surfer holds no token
func TestWithSignIn_WhenAControlNeedsASessionAndThereIsNoToken_SignsInBeforeSending(t *testing.T) {
	// Arrange
	a := newAuthServer(t)
	var calls atomic.Int32
	s := authSurfer(t, a, surf.WithSignIn(signInWith("good", &calls)))
	home, _ := s.Root(context.Background())

	// Act
	page, err := home.Follow(context.Background(), "protected")

	// Assert
	if err != nil || page.Status != 200 {
		t.Fatalf("Follow returned %v (page %+v), want it to sign in first and succeed", err, page)
	}
	if signIns, requests := a.counts(); signIns != 1 || requests != 1 {
		t.Errorf("server saw %d sign-ins and %d protected requests, want 1 and 1: no wasted request", signIns, requests)
	}
}

// Without the hook, behaviour is unchanged: no token means the step is refused before sending.
func TestWithoutSignIn_NoTokenIsStillRefusedBeforeSending(t *testing.T) {
	a := newAuthServer(t)
	s := authSurfer(t, a)
	home, _ := s.Root(context.Background())

	_, err := home.Follow(context.Background(), "protected")

	var notMet *hypertext.RequiresNotMetError
	if !errors.As(err, &notMet) {
		t.Errorf("Follow returned %v, want *RequiresNotMetError", err)
	}
	if _, requests := a.counts(); requests != 0 {
		t.Errorf("%d requests were sent, want none", requests)
	}
}

// A journey that tests the refusal itself must see it, not have it papered over.
func TestWithSignIn_AStepThatExpects401DoesNotSignIn(t *testing.T) {
	a := newAuthServer(t)
	var calls atomic.Int32
	s := authSurfer(t, a, surf.WithBearer("stale"), surf.WithSignIn(signInWith("good", &calls)))
	home, _ := s.Root(context.Background())

	page, err := home.Follow(context.Background(), "protected", surf.Expect(401))

	if err != nil || page.Status != 401 {
		t.Fatalf("an expected 401 should come back as a page, got %v / %+v", err, page)
	}
	if calls.Load() != 0 {
		t.Errorf("the hook ran %d times for an expected 401, want none", calls.Load())
	}
}

// One sign-in and one replay per step, never a loop: if the new token is refused too, say so.
func TestWithSignIn_WhenTheNewTokenIsRefusedToo_GivesUpAfterOneTry(t *testing.T) {
	a := newAuthServer(t)
	a.refuseNew = true
	var calls atomic.Int32
	s := authSurfer(t, a, surf.WithBearer("stale"), surf.WithSignIn(signInWith("good", &calls)))
	home, _ := s.Root(context.Background())

	_, err := home.Follow(context.Background(), "protected")

	var refused *hypertext.UnexpectedStatusError
	if !errors.As(err, &refused) || refused.Status != 401 {
		t.Fatalf("returned %v, want the second 401 as *UnexpectedStatusError", err)
	}
	if calls.Load() != 1 {
		t.Errorf("the hook ran %d times, want exactly 1", calls.Load())
	}
	if _, requests := a.counts(); requests != 2 {
		t.Errorf("%d protected requests were sent, want 2 (original and one replay)", requests)
	}
}

// Signing in is itself a step that can be refused. That refusal must not call the hook again.
func TestWithSignIn_WhenSigningInIsRefused_DoesNotRecurse(t *testing.T) {
	a := newAuthServer(t)
	var calls atomic.Int32
	s := authSurfer(t, a, surf.WithBearer("stale"), surf.WithSignIn(signInWith("wrong", &calls)))
	home, _ := s.Root(context.Background())

	_, err := home.Follow(context.Background(), "protected")

	if err == nil {
		t.Fatal("a failed sign-in should fail the step")
	}
	if calls.Load() != 1 {
		t.Errorf("the hook ran %d times, want exactly 1 (no recursion)", calls.Load())
	}
	var refused *hypertext.UnexpectedStatusError
	if !errors.As(err, &refused) {
		t.Errorf("the error %v should wrap the sign-in refusal so the cause can be read", err)
	}
}

func TestWithSignIn_WhenTheHookFails_TheStepReportsWhy(t *testing.T) {
	a := newAuthServer(t)
	boom := errors.New("the credential store is locked")
	s := authSurfer(t, a, surf.WithBearer("stale"), surf.WithSignIn(func(ctx context.Context, from *surf.Page) (string, error) {
		return "", boom
	}))
	home, _ := s.Root(context.Background())

	_, err := home.Follow(context.Background(), "protected")

	if !errors.Is(err, boom) {
		t.Errorf("returned %v, want it to wrap the hook's error", err)
	}
}

func TestWithSignIn_WhenTheHookReturnsNoToken_TheStepFails(t *testing.T) {
	a := newAuthServer(t)
	s := authSurfer(t, a, surf.WithBearer("stale"), surf.WithSignIn(func(ctx context.Context, from *surf.Page) (string, error) {
		return "", nil
	}))
	home, _ := s.Root(context.Background())

	_, err := home.Follow(context.Background(), "protected")

	if err == nil || !strings.Contains(err.Error(), "no token") {
		t.Errorf("returned %v, want an error saying the hook gave no token", err)
	}
}

// Many steps hit the same expired token at once. They share one sign-in, and all carry on.
func TestWithSignIn_ManyStepsAtOnce_ShareOneSignIn(t *testing.T) {
	for run := 0; run < 10; run++ {
		// Arrange
		a := newAuthServer(t)
		var calls atomic.Int32
		s := authSurfer(t, a, surf.WithBearer("stale"), surf.WithSignIn(signInWith("good", &calls)))
		home, _ := s.Root(context.Background())
		errs := make([]error, 6)
		var wg sync.WaitGroup

		// Act
		for i := range errs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, errs[i] = home.Follow(context.Background(), "protected")
			}()
		}
		wg.Wait()

		// Assert
		for i, err := range errs {
			if err != nil {
				t.Fatalf("run %d: step %d failed: %v", run, i, err)
			}
		}
		if signIns, _ := a.counts(); signIns != 1 || calls.Load() != 1 {
			t.Fatalf("run %d: %d sign-ins and %d hook calls for 6 concurrent steps, want 1 and 1", run, signIns, calls.Load())
		}
	}
}
