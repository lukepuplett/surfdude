package surf

import (
	"strings"

	"github.com/lukepuplett/surfdude/go/hypertext"
)

// preflight checks whether the surfer may take a control before any HTTP request is sent.
func preflight(c hypertext.Control, token string) error {
	if len(c.Requires) > 0 && token == "" {
		return &hypertext.RequiresNotMetError{Rel: c.Rel, Requires: c.Requires}
	}
	if c.Price != nil {
		return &hypertext.PaymentRequiredError{Rel: c.Rel, Price: c.Price}
	}
	if c.Type != "" && !strings.Contains(strings.ToLower(c.Type), "json") {
		return &hypertext.NonHypermediaControlError{Rel: c.Rel, MediaType: c.Type}
	}
	return nil
}
