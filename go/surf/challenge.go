package surf

import (
	"context"
	"fmt"
	"net/http"
	"sync"
)

// ChallengeMatcher says whether a response is a challenge Surfdude should handle instead of failing
// the step. Match runs on the raw response before the step's expected-status check settles.
type ChallengeMatcher func(status int, header http.Header, body []byte) bool

// ChallengeContext is what a handler receives: the refused page, the original request, and the surfer.
type ChallengeContext struct {
	Surfer *Surfer
	Step   string
	Req    *http.Request
	Page   *Page
}

// ChallengeHandler runs the side work (sign-in, sandbox credit, x402 pay, …). When it returns
// retry true, the surfer replays the original request once.
type ChallengeHandler func(ctx context.Context, cc ChallengeContext) (retry bool, err error)

// Challenge is one registered response pattern and how to recover from it.
type Challenge struct {
	Name    string
	Match   ChallengeMatcher
	Handle  ChallengeHandler
}

// WithChallenges appends challenge handlers, tried in order after the built-in 401 sign-in hook.
func WithChallenges(cs ...Challenge) Option {
	return func(s *Surfer) { s.challenges = append(s.challenges, cs...) }
}

type challengingKey struct{}

func challenging(ctx context.Context) bool {
	_, ok := ctx.Value(challengingKey{}).(bool)
	return ok
}

// tryChallenges runs the first matching handler. refused is the page the server sent.
func (s *Surfer) tryChallenges(ctx context.Context, req *http.Request, step string, refused *Page) (retry bool, err error) {
	if len(s.challenges) == 0 || challenging(ctx) {
		return false, nil
	}

	body := refused.Resource.Body
	for _, ch := range s.challenges {
		if ch.Match == nil || ch.Handle == nil {
			continue
		}
		if !ch.Match(refused.Status, refused.Header, body) {
			continue
		}
		retry, err := ch.Handle(context.WithValue(ctx, challengingKey{}, true), ChallengeContext{
			Surfer: s, Step: step, Req: req, Page: refused,
		})
		if err != nil {
			return false, fmt.Errorf("Challenge %q failed while %s: %w", ch.Name, step, err)
		}
		return retry, nil
	}
	return false, nil
}

// challengeMu serialises challenge handlers that change shared surfer state.
var challengeMu sync.Mutex
