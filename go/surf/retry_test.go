// Tests for the opt-in retry policy. Nothing is retried unless a policy is given, and a retry is
// step-aware: a status the step expects is never retried.
package surf_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/lukepuplett/surfdude/go/dialects/agenttrader"
	"github.com/lukepuplett/surfdude/go/hypertext"
	"github.com/lukepuplett/surfdude/go/surf"
)

// retryServer counts the requests to each path and lets a test script the answers.
type retryServer struct {
	*httptest.Server
	mu   sync.Mutex
	hits map[string]int
	keys map[string][]string
}

type script func(path string, hit int, w http.ResponseWriter, r *http.Request) bool

func (rs *retryServer) count(path string) int {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.hits[path]
}

func (rs *retryServer) keysAt(path string) []string {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return append([]string(nil), rs.keys[path]...)
}

func retryHome() map[string]any {
	return map[string]any{
		"status": 200, "message": "OK", "data": []any{},
		"_links": []any{
			map[string]any{"rel": "flaky", "href": "/flaky", "method": "GET"},
			map[string]any{"rel": "limited", "href": "/limited", "method": "GET"},
			map[string]any{"rel": "drop", "href": "/drop", "method": "GET"},
		},
		"_actions": []any{
			map[string]any{
				"rel": "shortlist", "action": "/shortlist", "method": "POST", "enctype": "application/json",
				"fields": []any{map[string]any{"name": "note", "required": false, "schema": map[string]any{"type": "string"}}},
			},
		},
	}
}

func busy(w http.ResponseWriter) {
	writeGapJSON(w, http.StatusServiceUnavailable, map[string]any{"status": 503, "message": "busy", "data": []any{}})
}

func newRetryServer(t *testing.T, sc script) *retryServer {
	t.Helper()
	rs := &retryServer{hits: map[string]int{}, keys: map[string][]string{}}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		rs.mu.Lock()
		rs.hits[path]++
		hit := rs.hits[path]
		rs.keys[path] = append(rs.keys[path], r.Header.Get("Idempotency-Key"))
		rs.mu.Unlock()

		if sc != nil && sc(path, hit, w, r) {
			return
		}
		if path == "/" {
			writeGapJSON(w, 200, retryHome())
			return
		}
		writeGapJSON(w, 200, map[string]any{"status": 200, "message": "OK", "data": []any{}})
	}))
	t.Cleanup(rs.Close)
	return rs
}

// failFirst answers 503 to the first n requests to path.
func failFirst(path string, n int) script {
	return func(p string, hit int, w http.ResponseWriter, _ *http.Request) bool {
		if p == path && hit <= n {
			busy(w)
			return true
		}
		return false
	}
}

type sleeper struct {
	mu    sync.Mutex
	waits []time.Duration
}

func (s *sleeper) sleep(ctx context.Context, d time.Duration) error {
	s.mu.Lock()
	s.waits = append(s.waits, d)
	s.mu.Unlock()
	return ctx.Err()
}

func steadyPolicy() *surf.StandardRetryPolicy {
	p := surf.StandardRetry()
	p.Jitter = func() float64 { return 1 }
	return p
}

func retrySurfer(t *testing.T, rs *retryServer, sl *sleeper, opts ...surf.Option) *surf.Surfer {
	t.Helper()
	all := []surf.Option{surf.WithDialects(agenttrader.New()), surf.WithSleep(sl.sleep)}
	s, err := surf.New(rs.URL, append(all, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func openHome(t *testing.T, s *surf.Surfer) *surf.Page {
	t.Helper()
	home, err := s.Root(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return home
}

func TestRetryDefaultIsNone(t *testing.T) {
	rs := newRetryServer(t, failFirst("/flaky", 1))
	s := retrySurfer(t, rs, &sleeper{})

	_, err := openHome(t, s).Follow(context.Background(), "flaky")

	var us *hypertext.UnexpectedStatusError
	if !errors.As(err, &us) || us.Status != 503 {
		t.Fatalf("a surfer with no policy should fail on the first 503, got %v", err)
	}
	if got := rs.count("/flaky"); got != 1 {
		t.Fatalf("nothing should be retried by default, but the server saw %d requests", got)
	}
}

func TestRetryAGetUntilItSucceedsWithBackoff(t *testing.T) {
	rs := newRetryServer(t, failFirst("/flaky", 2))
	sl := &sleeper{}
	s := retrySurfer(t, rs, sl, surf.WithRetry(steadyPolicy()))

	page, err := openHome(t, s).Follow(context.Background(), "flaky")
	if err != nil {
		t.Fatalf("the third attempt should have succeeded: %v", err)
	}

	if page.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3", page.Attempts)
	}
	want := []time.Duration{200 * time.Millisecond, 400 * time.Millisecond}
	if len(sl.waits) != 2 || sl.waits[0] != want[0] || sl.waits[1] != want[1] {
		t.Errorf("waits = %v, want %v (exponential backoff)", sl.waits, want)
	}
}

func TestRetryGivesUpAfterMaxAttempts(t *testing.T) {
	rs := newRetryServer(t, failFirst("/flaky", 99))
	p := steadyPolicy()
	p.MaxAttempts = 2
	s := retrySurfer(t, rs, &sleeper{}, surf.WithRetry(p))

	_, err := openHome(t, s).Follow(context.Background(), "flaky")

	var us *hypertext.UnexpectedStatusError
	if !errors.As(err, &us) {
		t.Fatalf("want an UnexpectedStatusError once the attempts run out, got %v", err)
	}
	if us.Attempts != 2 || rs.count("/flaky") != 2 {
		t.Errorf("Attempts = %d and the server saw %d requests, want 2 and 2", us.Attempts, rs.count("/flaky"))
	}
}

func TestRetryNeverRetriesAStatusTheStepExpects(t *testing.T) {
	rs := newRetryServer(t, failFirst("/flaky", 99))
	s := retrySurfer(t, rs, &sleeper{}, surf.WithRetry(steadyPolicy()))

	page, err := openHome(t, s).Follow(context.Background(), "flaky", surf.Expect(503))
	if err != nil {
		t.Fatalf("503 was expected, so it is not a failure: %v", err)
	}

	if page.Attempts != 1 || rs.count("/flaky") != 1 {
		t.Fatalf("a journey that expects 503 must see it once, but there were %d attempts", rs.count("/flaky"))
	}
}

func TestRetryAStepCanTurnTheSurferPolicyOff(t *testing.T) {
	rs := newRetryServer(t, failFirst("/flaky", 1))
	s := retrySurfer(t, rs, &sleeper{}, surf.WithRetry(steadyPolicy()))

	_, err := openHome(t, s).Follow(context.Background(), "flaky", surf.Retrying(surf.Never()))

	if err == nil || rs.count("/flaky") != 1 {
		t.Fatalf("Retrying(Never()) should fail on the first 503; err=%v hits=%d", err, rs.count("/flaky"))
	}
}

func TestRetryAStepCanTurnRetriesOnForASurferWithNone(t *testing.T) {
	rs := newRetryServer(t, failFirst("/flaky", 1))
	s := retrySurfer(t, rs, &sleeper{})

	page, err := openHome(t, s).Follow(context.Background(), "flaky", surf.Retrying(steadyPolicy()))

	if err != nil || page.Attempts != 2 {
		t.Fatalf("the step's own policy should have retried once: err=%v", err)
	}
}

func TestRetryDoesNotRepeatAPostByDefault(t *testing.T) {
	rs := newRetryServer(t, failFirst("/shortlist", 1))
	s := retrySurfer(t, rs, &sleeper{}, surf.WithRetry(steadyPolicy()))

	_, err := openHome(t, s).Submit(context.Background(), "shortlist", surf.Values{"note": "x"})

	if err == nil || rs.count("/shortlist") != 1 {
		t.Fatalf("a POST without an idempotency key must not be repeated; err=%v hits=%d", err, rs.count("/shortlist"))
	}
	if keys := rs.keysAt("/shortlist"); len(keys) != 1 || keys[0] != "" {
		t.Errorf("no Idempotency-Key should be sent unless asked for, got %q", keys)
	}
}

func TestRetryRepeatsAPostThatCarriesAnIdempotencyKey(t *testing.T) {
	rs := newRetryServer(t, failFirst("/shortlist", 1))
	s := retrySurfer(t, rs, &sleeper{}, surf.WithRetry(steadyPolicy()))

	page, err := openHome(t, s).Submit(context.Background(), "shortlist", surf.Values{"note": "x"}, surf.WithIdempotencyKey())
	if err != nil {
		t.Fatalf("the repeat should have succeeded: %v", err)
	}

	keys := rs.keysAt("/shortlist")
	if page.Attempts != 2 || len(keys) != 2 {
		t.Fatalf("want 2 attempts, got Attempts=%d and %d requests", page.Attempts, len(keys))
	}
	if keys[0] == "" || keys[0] != keys[1] {
		t.Errorf("both attempts must carry the same non-empty key, got %q", keys)
	}
}

func TestRetryHonoursRetryAfterSeconds(t *testing.T) {
	rs := newRetryServer(t, func(path string, hit int, w http.ResponseWriter, _ *http.Request) bool {
		if path == "/limited" && hit == 1 {
			w.Header().Set("Retry-After", "2")
			writeGapJSON(w, 429, map[string]any{"status": 429, "message": "slow down", "data": []any{}})
			return true
		}
		return false
	})
	sl := &sleeper{}
	s := retrySurfer(t, rs, sl, surf.WithRetry(steadyPolicy()))

	if _, err := openHome(t, s).Follow(context.Background(), "limited"); err != nil {
		t.Fatal(err)
	}

	if len(sl.waits) != 1 || sl.waits[0] != 2*time.Second {
		t.Fatalf("waits = %v, want the server's 2s and not the backoff", sl.waits)
	}
}

func TestRetryDoesNotWaitForARetryAfterBeyondTheCap(t *testing.T) {
	rs := newRetryServer(t, func(path string, hit int, w http.ResponseWriter, _ *http.Request) bool {
		if path == "/limited" {
			w.Header().Set("Retry-After", "120")
			writeGapJSON(w, 429, map[string]any{"status": 429, "message": "slow down", "data": []any{}})
			return true
		}
		return false
	})
	sl := &sleeper{}
	s := retrySurfer(t, rs, sl, surf.WithRetry(steadyPolicy()))

	_, err := openHome(t, s).Follow(context.Background(), "limited")

	if err == nil || len(sl.waits) != 0 || rs.count("/limited") != 1 {
		t.Fatalf("a two minute wait exceeds the 5s cap, so it should fail at once; err=%v waits=%v", err, sl.waits)
	}
}

func TestRetryRepeatsAfterAConnectionError(t *testing.T) {
	rs := newRetryServer(t, func(path string, hit int, w http.ResponseWriter, _ *http.Request) bool {
		if path == "/drop" && hit == 1 {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
			return true
		}
		return false
	})
	// A fresh connection per request, so the standard library does not quietly retry for us.
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	s := retrySurfer(t, rs, &sleeper{}, surf.WithHTTPClient(client), surf.WithRetry(steadyPolicy()))

	page, err := openHome(t, s).Follow(context.Background(), "drop")
	if err != nil {
		t.Fatalf("the second attempt should have succeeded: %v", err)
	}

	if page.Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", page.Attempts)
	}
}

func TestRetryStopsWhenTheContextIsCancelledWhileWaiting(t *testing.T) {
	rs := newRetryServer(t, failFirst("/flaky", 99))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelling := func(ctx context.Context, _ time.Duration) error {
		cancel()
		return ctx.Err()
	}
	s, err := surf.New(rs.URL, surf.WithDialects(agenttrader.New()), surf.WithSleep(cancelling), surf.WithRetry(steadyPolicy()))
	if err != nil {
		t.Fatal(err)
	}
	home, err := s.Root(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = home.Follow(ctx, "flaky")

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if rs.count("/flaky") != 1 {
		t.Errorf("no further attempt should follow a cancellation, but the server saw %d", rs.count("/flaky"))
	}
}

func TestRetryAttemptsAppearInTheWaveReport(t *testing.T) {
	rs := newRetryServer(t, failFirst("/flaky", 2))
	s := retrySurfer(t, rs, &sleeper{}, surf.WithRetry(steadyPolicy()))

	report, err := surf.NewWave(s).FromRoot().Then("flaky").Go(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if got := report.Lines[1].Attempts; got != 3 {
		t.Errorf("the flaky step reports %d attempts, want 3", got)
	}
}

func TestRetryTheReportShowsTheFailingStepAndItsAttempts(t *testing.T) {
	rs := newRetryServer(t, failFirst("/flaky", 99))
	p := steadyPolicy()
	p.MaxAttempts = 2
	s := retrySurfer(t, rs, &sleeper{}, surf.WithRetry(p))

	report, err := surf.NewWave(s).FromRoot().Then("flaky").Go(context.Background())

	if err == nil {
		t.Fatal("the wave should have stopped")
	}
	last := report.Lines[len(report.Lines)-1]
	if last.Status != 503 || last.Attempts != 2 {
		t.Errorf("the failing line shows status %d and %d attempts, want 503 and 2", last.Status, last.Attempts)
	}
}

func TestTimeoutIsConfigurable(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		writeGapJSON(w, 200, retryHome())
	}))
	defer slow.Close()
	s, err := surf.New(slow.URL, surf.WithDialects(agenttrader.New()), surf.WithTimeout(30*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	_, err = s.Root(context.Background())

	if err == nil {
		t.Fatal("a 30ms timeout should have failed a 300ms response")
	}
	if took := time.Since(start); took > 200*time.Millisecond {
		t.Errorf("the request ran for %v, so the timeout was not applied", took)
	}
}

func TestSetBearerIsSafeWhileRequestsAreInFlight(t *testing.T) {
	rs := newRetryServer(t, nil)
	s := retrySurfer(t, rs, &sleeper{})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				s.SetBearer("token")
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_, _ = s.Root(context.Background())
			}
		}()
	}
	wg.Wait() // `go test -race` fails here while the token is read and written without a lock
}
