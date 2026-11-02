package surf

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RetryAttempt describes an attempt that has just failed, so a RetryPolicy can decide what to do.
type RetryAttempt struct {
	// Number is how many attempts have been made so far, counting from 1.
	Number int
	// Method is the HTTP method of the request.
	Method string
	// Status is the status the server answered with, or 0 when the request itself failed.
	Status int
	// Header holds the response headers, or nil when the request itself failed.
	Header http.Header
	// Err is the transport error, or nil when the server answered.
	Err error
	// Idempotent is true when repeating the request is safe: the method is idempotent, or the request
	// carries an Idempotency-Key.
	Idempotent bool
}

// RetryPolicy decides whether a failed attempt is tried again, and how long to wait first.
//
// A surfer retries nothing unless it is given a policy, because a tool that tests an API should see
// every failure. A client that drives a real server can opt in with WithRetry or Retrying.
type RetryPolicy interface {
	Next(RetryAttempt) (wait time.Duration, retry bool)
}

// RetryFunc turns a function into a RetryPolicy.
type RetryFunc func(RetryAttempt) (time.Duration, bool)

// Next calls f.
func (f RetryFunc) Next(a RetryAttempt) (time.Duration, bool) { return f(a) }

// Never is the policy that retries nothing. It is what a surfer uses when it has no policy, and it is
// how one step turns off a surfer-wide policy.
func Never() RetryPolicy {
	return RetryFunc(func(RetryAttempt) (time.Duration, bool) { return 0, false })
}

// StandardRetryPolicy is a conservative preset for clients of real servers.
type StandardRetryPolicy struct {
	// MaxAttempts is the total number of attempts, including the first. Zero means 3.
	MaxAttempts int
	// BaseDelay is the backoff before the second attempt; it doubles each time. Zero means 200ms.
	BaseDelay time.Duration
	// MaxDelay caps the backoff. A Retry-After longer than this is not waited for: the attempt fails
	// instead. Zero means 5s.
	MaxDelay time.Duration
	// Statuses are the response statuses worth another try. Nil means 429, 502, 503 and 504.
	Statuses []int
	// Jitter returns a number in [0, 1) that scales each backoff. Nil means a random number.
	Jitter func() float64
}

// StandardRetry returns the preset: three attempts, exponential backoff with jitter from 200ms up to
// 5s, on 429, 502, 503, 504 and connection errors, for idempotent requests only.
func StandardRetry() *StandardRetryPolicy {
	return &StandardRetryPolicy{}
}

// Next implements RetryPolicy.
func (p *StandardRetryPolicy) Next(a RetryAttempt) (time.Duration, bool) {
	if a.Number >= orDefault(p.MaxAttempts, 3) || !a.Idempotent {
		return 0, false
	}
	if a.Err == nil && !p.retriesStatus(a.Status) {
		return 0, false
	}

	maxDelay := orDefaultDuration(p.MaxDelay, 5*time.Second)
	if wait, ok := retryAfter(a.Header); ok {
		// The server said when to come back. Coming back sooner is rude, and waiting too long
		// stalls the caller, so a request for more than the cap is treated as a refusal.
		return wait, wait <= maxDelay
	}

	backoff := orDefaultDuration(p.BaseDelay, 200*time.Millisecond)
	for i := 1; i < a.Number && backoff < maxDelay; i++ {
		backoff *= 2
	}
	if backoff > maxDelay {
		backoff = maxDelay
	}

	jitter := p.Jitter
	if jitter == nil {
		jitter = rand.Float64
	}
	return time.Duration(jitter() * float64(backoff)), true
}

func (p *StandardRetryPolicy) retriesStatus(status int) bool {
	statuses := p.Statuses
	if statuses == nil {
		statuses = []int{http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout}
	}
	for _, s := range statuses {
		if s == status {
			return true
		}
	}
	return false
}

// retryAfter reads a Retry-After header, either a number of seconds or an HTTP date.
func retryAfter(h http.Header) (time.Duration, bool) {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		if wait := time.Until(t); wait > 0 {
			return wait, true
		}
		return 0, true
	}
	return 0, false
}

func orDefault(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}

func orDefaultDuration(v, fallback time.Duration) time.Duration {
	if v == 0 {
		return fallback
	}
	return v
}

// idempotent reports whether repeating the request is safe.
func idempotent(req *http.Request) bool {
	switch req.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete, http.MethodTrace:
		return true
	}
	return req.Header.Get("Idempotency-Key") != ""
}

// replayable reports whether the request can be sent again: it has no body, or a way to rebuild it.
func replayable(req *http.Request) bool {
	return req.Body == nil || req.Body == http.NoBody || req.GetBody != nil
}

// rewind returns a copy of the request with a fresh body, ready to send again.
func rewind(req *http.Request) (*http.Request, error) {
	next := req.Clone(req.Context())
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		next.Body = body
	}
	return next, nil
}

// newIdempotencyKey returns a random key for the Idempotency-Key header.
func newIdempotencyKey() (string, error) {
	b := make([]byte, 16)
	if _, err := crand.Read(b); err != nil {
		return "", fmt.Errorf("Unable to make an idempotency key: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// sleepContext waits for d, or until the context ends.
func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
