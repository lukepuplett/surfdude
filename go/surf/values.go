package surf

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/lukepuplett/surfdude/go/hypertext"
)

// Values are what a journey sends to a form, keyed by input name. A dotted name such as
// "location.lat" becomes a nested object in a JSON body.
type Values map[string]any

// merge checks the caller's values against the control and returns what to send: the caller's values
// plus any prefilled values they did not override. It fails with an error that names the problem
// and what the control does accept.
func merge(c hypertext.Control, v Values) (map[string]any, error) {
	return mergeWith(c, v, false)
}

// mergeWith is merge, and with unchecked set it skips every check: an unknown field, a changed
// read-only input, a missing required field and a value outside its schema all go through, and the
// caller's values are sent exactly as given. Prefilled values are still merged in, because that is
// what the form offers, not a check.
func mergeWith(c hypertext.Control, v Values, unchecked bool) (map[string]any, error) {
	allowed := map[string]bool{}
	for _, in := range c.Inputs {
		allowed[in.Name] = true
	}
	for _, name := range templateVars(c.Href) {
		allowed[name] = true
	}

	if !unchecked {
		for name := range v {
			if !allowed[name] {
				return nil, &hypertext.UnexpectedInputError{Rel: c.Rel, Name: name, Allowed: sortedKeys(allowed)}
			}
		}

		// A read-only input is the API saying "this is decided". The value it offered is sent, and a
		// caller may repeat it but not change it. With nothing offered there is nothing to repeat.
		for _, in := range c.Inputs {
			if given, ok := v[in.Name]; ok && in.ReadOnly() && (in.Value == nil || !sameValue(in.Value, given)) {
				return nil, &hypertext.ReadOnlyInputError{Rel: c.Rel, Name: in.Name}
			}
		}
	}

	out := map[string]any{}
	for _, in := range c.Inputs {
		if in.Value != nil {
			out[in.Name] = in.Value
		}
	}
	for name, val := range v {
		out[name] = val
	}

	if unchecked {
		return out, nil
	}

	var missing []hypertext.Input
	for _, in := range c.Inputs {
		if _, ok := out[in.Name]; in.Required && !ok {
			missing = append(missing, in)
		}
	}
	if len(missing) > 0 {
		return nil, &hypertext.MissingInputError{Rel: c.Rel, Missing: missing}
	}

	for _, in := range c.Inputs {
		if val, ok := out[in.Name]; ok {
			if reason := checkSchema(in.Schema, val); reason != "" {
				return nil, &hypertext.InvalidInputError{Rel: c.Rel, Name: in.Name, Reason: reason}
			}
		}
	}

	return out, nil
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// checkSchema returns why a value does not fit a JSON Schema fragment, or "". It checks type, enum,
// minimum, maximum, minLength, maxLength and pattern, which is what the forms we meet use.
func checkSchema(schema hypertext.Schema, val any) string {
	if len(schema) == 0 {
		return ""
	}

	if t, ok := schema["type"].(string); ok {
		if reason := checkType(t, val); reason != "" {
			return reason
		}
	}

	if format, ok := schema["format"].(string); ok {
		if reason := checkFormat(format, val); reason != "" {
			return reason
		}
	}

	if enum, ok := schema["enum"].([]any); ok && len(enum) > 0 {
		found := false
		for _, e := range enum {
			if sameValue(e, val) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Sprintf("must be one of %s", joinValues(enum))
		}
	}

	if n, ok := toFloat(val); ok {
		if min, ok := toFloat(schema["minimum"]); ok && n < min {
			return fmt.Sprintf("must be at least %v", min)
		}
		if max, ok := toFloat(schema["maximum"]); ok && n > max {
			return fmt.Sprintf("must be at most %v", max)
		}
	}

	if s, ok := val.(string); ok {
		length := utf8.RuneCountInString(s)
		if min, ok := toFloat(schema["minLength"]); ok && float64(length) < min {
			return fmt.Sprintf("must be at least %v characters", min)
		}
		if max, ok := toFloat(schema["maxLength"]); ok && float64(length) > max {
			return fmt.Sprintf("must be at most %v characters", max)
		}
		if pattern, ok := schema["pattern"].(string); ok {
			if re, err := regexp.Compile(pattern); err == nil && !re.MatchString(s) {
				return fmt.Sprintf("must match %s", pattern)
			}
		}
	}

	return ""
}

var emailFormat = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func checkFormat(format string, val any) string {
	if format != "email" {
		return ""
	}
	s, ok := val.(string)
	if !ok {
		return "must be a valid email address"
	}
	if !emailFormat.MatchString(s) {
		return "must be a valid email address"
	}
	return ""
}

func checkType(t string, val any) string {
	switch t {
	case "string":
		if _, ok := val.(string); !ok {
			return "must be text"
		}
	case "integer":
		if f, ok := toFloat(val); !ok || f != float64(int64(f)) {
			return "must be a whole number"
		}
	case "number":
		if _, ok := toFloat(val); !ok {
			return "must be a number"
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			return "must be true or false"
		}
	}
	return ""
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func sameValue(a, b any) bool {
	if fa, ok := toFloat(a); ok {
		if fb, ok := toFloat(b); ok {
			return fa == fb
		}
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}

func joinValues(vs []any) string {
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = fmt.Sprint(v)
	}
	return strings.Join(parts, ", ")
}

// nest turns dotted keys into nested objects: {"location.lat": 1} becomes {"location": {"lat": 1}}.
func nest(flat map[string]any) map[string]any {
	out := map[string]any{}
	for key, val := range flat {
		parts := strings.Split(key, ".")
		cur := out
		for i, part := range parts {
			if i == len(parts)-1 {
				cur[part] = val
				break
			}
			next, ok := cur[part].(map[string]any)
			if !ok {
				next = map[string]any{}
				cur[part] = next
			}
			cur = next
		}
	}
	return out
}

var templatePart = regexp.MustCompile(`\{(\?)?([^}]+)\}`)

// templateVars lists the variables in a URI template: {name} and {?a,b,c}.
func templateVars(href string) []string {
	var out []string
	for _, m := range templatePart.FindAllStringSubmatch(href, -1) {
		for _, name := range strings.Split(m[2], ",") {
			out = append(out, strings.TrimSpace(name))
		}
	}
	return out
}

// expand fills a URI template. {name} is a path variable and {?a,b} adds only the query variables
// that were given. It returns the address and the values it used, so the rest can go in the body or
// query.
func expand(href string, vals map[string]any) (string, map[string]bool) {
	used := map[string]bool{}
	out := templatePart.ReplaceAllStringFunc(href, func(match string) string {
		m := templatePart.FindStringSubmatch(match)
		names := strings.Split(m[2], ",")
		if m[1] == "?" {
			q := url.Values{}
			for _, name := range names {
				name = strings.TrimSpace(name)
				if v, ok := vals[name]; ok {
					used[name] = true
					q.Set(name, fmt.Sprint(v))
				}
			}
			if len(q) == 0 {
				return ""
			}
			return "?" + q.Encode()
		}
		name := strings.TrimSpace(names[0])
		if v, ok := vals[name]; ok {
			used[name] = true
			return url.PathEscape(fmt.Sprint(v))
		}
		return match
	})
	return out, used
}
