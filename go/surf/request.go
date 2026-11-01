package surf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/lukepuplett/surfdude/go/dialect"
	"github.com/lukepuplett/surfdude/go/hypertext"
)

// buildRequest turns a control and the values for it into an HTTP request. Nothing in a journey
// names a path or a method: both come from the control.
func buildRequest(ctx context.Context, reg *dialect.Registry, base *url.URL, c hypertext.Control, vals map[string]any, token string) (*http.Request, error) {
	target, used := expand(c.Href, vals)
	u, err := base.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("Unable to follow %q. Its address %q is not a valid URL: %w", c.Rel, c.Href, err)
	}

	rest := map[string]any{}
	for k, v := range vals {
		if !used[k] {
			rest[k] = v
		}
	}

	method := c.HTTPMethod()
	var body io.Reader
	contentType := ""

	switch method {
	case http.MethodGet, http.MethodHead, http.MethodDelete:
		if len(rest) > 0 {
			q := u.Query()
			for k, v := range rest {
				addQuery(q, k, v)
			}
			u.RawQuery = q.Encode()
		}
	default:
		if len(rest) > 0 {
			enc := c.Enctype
			if enc == "" {
				if d, ok := reg.First(); ok {
					enc = d.BodyType()
				}
			}
			switch {
			case strings.HasPrefix(enc, "application/x-www-form-urlencoded"):
				form := url.Values{}
				for k, v := range rest {
					addQuery(form, k, v)
				}
				body, contentType = strings.NewReader(form.Encode()), "application/x-www-form-urlencoded"
			default:
				raw, err := json.Marshal(nest(rest))
				if err != nil {
					return nil, fmt.Errorf("Unable to submit %q. The values cannot be encoded as JSON: %w", c.Rel, err)
				}
				if enc == "" {
					enc = "application/json"
				}
				body, contentType = bytes.NewReader(raw), enc
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if accept := reg.Accept(); accept != "" {
		req.Header.Set("Accept", accept)
	}
	if c.IfMatch != "" {
		req.Header.Set("If-Match", c.IfMatch)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req, nil
}

func addQuery(q url.Values, key string, val any) {
	switch v := val.(type) {
	case []string:
		for _, s := range v {
			q.Add(key, s)
		}
	case []any:
		for _, s := range v {
			q.Add(key, fmt.Sprint(s))
		}
	default:
		q.Set(key, fmt.Sprint(val))
	}
}
