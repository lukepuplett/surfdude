// Package surftest lets a Go test drive an API with surf and fail with the helpful error when a step
// cannot be taken.
package surftest

import (
	"context"
	"testing"

	"github.com/lukepuplett/surfdude/go/hypertext"
	"github.com/lukepuplett/surfdude/go/surf"
)

// New builds a Surfer for the front door URL, failing the test if it cannot.
func New(t testing.TB, root string, opts ...surf.Option) *surf.Surfer {
	t.Helper()
	s, err := surf.New(root, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Root opens the front door, failing the test on error.
func Root(t testing.TB, s *surf.Surfer) *surf.Page {
	t.Helper()
	p, err := s.Root(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Follow takes a control, failing the test on error.
func Follow(t testing.TB, p *surf.Page, rel string, opts ...surf.Step) *surf.Page {
	t.Helper()
	next, err := p.Follow(context.Background(), rel, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return next
}

// Submit submits a form, failing the test on error.
func Submit(t testing.TB, p *surf.Page, rel string, v surf.Values, opts ...surf.Step) *surf.Page {
	t.Helper()
	next, err := p.Submit(context.Background(), rel, v, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return next
}

// Refused submits a form that the server should refuse, and returns the problems it reported. The
// test fails if the server accepted it.
func Refused(t testing.TB, p *surf.Page, rel string, v surf.Values, status int) []hypertext.Problem {
	t.Helper()
	return RefusedPage(t, p, rel, v, status).Resource.Problems
}

// RefusedPage is Refused, and it returns the whole refusal page, so a journey can read what the
// refusal offers next and its headers, such as Retry-After. The test fails if the server answered
// with any other status.
func RefusedPage(t testing.TB, p *surf.Page, rel string, v surf.Values, status int) *surf.Page {
	t.Helper()
	next, err := p.Submit(context.Background(), rel, v, surf.Expect(status))
	if err != nil {
		t.Fatal(err)
	}
	return next
}
