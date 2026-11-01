// Package dialect is the pluggable seam. A server spells its links and forms however it likes: a
// dialect knows one spelling and reads it into the neutral model in package hypertext. The surf
// package never sees raw JSON, so supporting a new server means writing a new dialect and nothing
// else.
package dialect

import (
	"net/http"
	"strings"

	"github.com/lukepuplett/surfdude/go/hypertext"
)

// Dialect reads one hypermedia spelling.
type Dialect interface {
	// Name identifies the dialect in reports and errors.
	Name() string
	// Accept is the media type to ask for.
	Accept() string
	// BodyType is the media type to send request bodies in, when a control does not say.
	BodyType() string
	// Matches says whether this dialect can read the response. It may use headers (a profile link,
	// a content type) and the body. It must not change either.
	Matches(resp *http.Response, body []byte) bool
	// Read turns the response into the neutral model.
	Read(resp *http.Response, body []byte) (*hypertext.Resource, error)
}

// Registry holds the dialects a client understands, tried in order.
type Registry struct {
	dialects []Dialect
}

// NewRegistry builds a registry. Earlier dialects win when more than one matches.
func NewRegistry(ds ...Dialect) *Registry {
	return &Registry{dialects: append([]Dialect(nil), ds...)}
}

// Add appends a dialect.
func (r *Registry) Add(d Dialect) { r.dialects = append(r.dialects, d) }

// Names lists the dialects in order.
func (r *Registry) Names() []string {
	out := make([]string, len(r.dialects))
	for i, d := range r.dialects {
		out[i] = d.Name()
	}
	return out
}

// Accept is the Accept header value: every dialect's media type, in order.
func (r *Registry) Accept() string {
	seen := map[string]bool{}
	var parts []string
	for _, d := range r.dialects {
		if a := d.Accept(); a != "" && !seen[a] {
			seen[a] = true
			parts = append(parts, a)
		}
	}
	return strings.Join(parts, ", ")
}

// Choose finds the first dialect that can read the response.
func (r *Registry) Choose(resp *http.Response, body []byte) (Dialect, bool) {
	for _, d := range r.dialects {
		if d.Matches(resp, body) {
			return d, true
		}
	}
	return nil, false
}

// First is the dialect used for request bodies, which is the first one registered.
func (r *Registry) First() (Dialect, bool) {
	if len(r.dialects) == 0 {
		return nil, false
	}
	return r.dialects[0], true
}

// Unsupported builds the error for a response nothing could read.
func (r *Registry) Unsupported(resp *http.Response) error {
	return &hypertext.UnsupportedMediaTypeError{ContentType: resp.Header.Get("Content-Type"), Tried: r.Names()}
}
