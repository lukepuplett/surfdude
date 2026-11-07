package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const secretMask = "<secret>"

// replacement is what to show in place of a field's value, if it must not be shown: the text in the
// mask (a secret, or the file a value came from), or a secret mask for a field whose name is on the
// hidden list, which is the safety net for a value the command did not know was secret. --reveal
// (in m) turns off only that second reason: the mask, which comes from the value's source, stays.
func replacement(name string, mask map[string]string, m masking) (string, bool) {
	if r, ok := mask[name]; ok {
		return r, true
	}
	if m.hides(name) {
		return secretMask, true
	}

	return "", false
}

// maskedURL is a request address with the values of masked query fields replaced, for showing.
func maskedURL(u *url.URL, mask map[string]string, m masking) string {
	shown := *u
	q := shown.Query()
	changed := false
	for name := range q {
		if r, ok := replacement(name, mask, m); ok {
			q.Set(name, r)
			changed = true
		}
	}
	if changed {
		shown.RawQuery = q.Encode()
	}

	return shown.String()
}

// maskBody is a request body as text for showing, with masked fields replaced. A JSON body is
// walked, so a nested field such as location.lat is found by its dotted name. A body that is neither
// JSON nor a form is not shown at all, only its size, because it cannot be searched for secrets.
func maskBody(contentType string, body []byte, mask map[string]string, m masking) string {
	mediaType, _, _ := mime.ParseMediaType(contentType)

	switch {
	case mediaType == "application/json" || strings.HasSuffix(mediaType, "+json"):
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.UseNumber()
		var v any
		if dec.Decode(&v) == nil {
			return marshalPlain(maskValue(v, "", mask, m))
		}

	case mediaType == "application/x-www-form-urlencoded":
		if q, err := url.ParseQuery(string(body)); err == nil {
			for name := range q {
				if r, ok := replacement(name, mask, m); ok {
					q.Set(name, r)
				}
			}
			return q.Encode()
		}
	}

	return fmt.Sprintf("<body of %d bytes, %s>", len(body), contentType)
}

// maskValue returns a copy of a decoded JSON value with masked fields replaced.
func maskValue(v any, path string, mask map[string]string, m masking) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, child := range x {
			full := k
			if path != "" {
				full = path + "." + k
			}
			if r, ok := replacement(full, mask, m); ok {
				out[k] = r
				continue
			}
			out[k] = maskValue(child, full, mask, m)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, child := range x {
			out[i] = maskValue(child, path, mask, m)
		}
		return out
	default:
		return v
	}
}

// marshalPlain writes JSON without escaping < and >, so a mask reads as <secret>.
func marshalPlain(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "<unprintable>"
	}

	return strings.TrimSpace(buf.String())
}

// printPlan shows the request a step would send: where, the headers without the token, and the body
// with secrets hidden. The Authorization header is always hidden, --reveal or not: the bearer token is
// not a field of the form, and it is the one value that would sign someone in.
func printPlan(w io.Writer, rel string, req *http.Request, mask map[string]string, m masking) {
	fmt.Fprintf(w, "%s %s -> %s\n", req.Method, rel, maskedURL(req.URL, mask, m))

	names := make([]string, 0, len(req.Header))
	for name := range req.Header {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := strings.Join(req.Header.Values(name), ", ")
		if name == "Authorization" {
			value = "Bearer " + secretMask
		}
		fmt.Fprintf(w, "  %s: %s\n", name, value)
	}

	if req.GetBody != nil {
		if rc, err := req.GetBody(); err == nil {
			raw, _ := io.ReadAll(rc)
			_ = rc.Close()
			if len(raw) > 0 {
				fmt.Fprintf(w, "  body: %s\n", maskBody(req.Header.Get("Content-Type"), raw, mask, m))
			}
		}
	}
}
