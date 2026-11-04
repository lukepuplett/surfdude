package hypertext

import (
	"fmt"
	"net/http"
	"strings"
)

// Every error reads "Unable to {action}. {reason}." and, where it can, says what to do instead, so a
// journey that cannot find its next step tells you what the response did offer.

// RelNotFoundError means the page has no control with that rel.
type RelNotFoundError struct {
	Rel     string
	Where   string
	Offered []string
}

func (e *RelNotFoundError) Error() string {
	offered := "nothing"
	if len(e.Offered) > 0 {
		offered = strings.Join(e.Offered, ", ")
	}
	return fmt.Sprintf("Unable to follow %q. %s has no control with that rel. It offers: %s.", e.Rel, e.Where, offered)
}

// MissingInputError means a form needs values the caller did not give.
type MissingInputError struct {
	Rel     string
	Missing []Input
}

func (e *MissingInputError) Error() string {
	parts := make([]string, len(e.Missing))
	for i, in := range e.Missing {
		parts[i] = in.Name
		if d, ok := in.Schema["description"].(string); ok && d != "" {
			parts[i] += " (" + d + ")"
		}
	}
	return fmt.Sprintf("Unable to submit %q. It needs: %s.", e.Rel, strings.Join(parts, ", "))
}

// InvalidInputError means a value does not fit what the form says is allowed.
type InvalidInputError struct {
	Rel    string
	Name   string
	Reason string
}

func (e *InvalidInputError) Error() string {
	return fmt.Sprintf("Unable to submit %q. The value for %s is not allowed: %s.", e.Rel, e.Name, e.Reason)
}

// UnexpectedInputError means a value was given for an input the form does not have.
type UnexpectedInputError struct {
	Rel     string
	Name    string
	Allowed []string
}

func (e *UnexpectedInputError) Error() string {
	return fmt.Sprintf("Unable to submit %q. It has no input called %s. Its inputs are: %s.", e.Rel, e.Name, strings.Join(e.Allowed, ", "))
}

// UnsupportedMediaTypeError means no dialect could read the response.
type UnsupportedMediaTypeError struct {
	ContentType string
	Tried       []string
}

func (e *UnsupportedMediaTypeError) Error() string {
	return fmt.Sprintf("Unable to read the response. No dialect understands %q. Dialects tried: %s.", e.ContentType, strings.Join(e.Tried, ", "))
}

// UnexpectedStatusError means the server answered with a status the step did not expect. Resource is
// the response read into the model, when a dialect could read it, so a test can look at the
// problems the server reported. Header and Body are the response as it arrived, whether or not a
// dialect could read it, so a test can read Retry-After or WWW-Authenticate, or see the plain-text
// page a proxy sent, from the error alone.
type UnexpectedStatusError struct {
	Step     string
	Status   int
	Message  string
	Resource *Resource
	Header   http.Header
	Body     []byte
	// Attempts is how many times the request was sent, when a retry policy tried more than once.
	Attempts int
}

func (e *UnexpectedStatusError) Error() string {
	text := fmt.Sprintf("Unable to %s. The server answered %d", e.Step, e.Status)
	if e.Message != "" {
		text += ": " + e.Message
	}
	if e.Resource != nil && len(e.Resource.Problems) > 0 {
		var ps []string
		for _, p := range e.Resource.Problems {
			ps = append(ps, strings.TrimSpace(p.Field+" "+p.Message))
		}
		text += " (" + strings.Join(ps, "; ") + ")"
	}
	text += "."
	if e.Attempts > 1 {
		text += fmt.Sprintf(" It was tried %d times.", e.Attempts)
	}
	return text
}

// NoDataError means the resource carried no data to read.
type NoDataError struct{}

func (e *NoDataError) Error() string { return "Unable to read data. The response carried none." }

// MultiItemReadError means Read was called on a list page without selecting an item.
type MultiItemReadError struct {
	Count int
}

func (e *MultiItemReadError) Error() string {
	return fmt.Sprintf("Unable to read data. This page has %d items; select one with Item(i).", e.Count)
}
