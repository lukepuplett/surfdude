package cli

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// lookup reads one value out of a JSON response by a dotted path such as data.0.token. A number in
// the path indexes a list. A string comes back as it is, without quotes, a number as it was written
// (a large one is not rounded), a boolean as true or false, null as nothing, and an object or a list
// as compact JSON. The second result is false when the body is not JSON or the path leads nowhere.
func lookup(body []byte, path string) (string, bool) {
	if path == "" {
		return "", false
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var cur any
	if err := dec.Decode(&cur); err != nil {
		return "", false
	}

	for _, part := range strings.Split(path, ".") {
		switch node := cur.(type) {
		case map[string]any:
			next, ok := node[part]
			if !ok {
				return "", false
			}
			cur = next
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(node) {
				return "", false
			}
			cur = node[i]
		default:
			return "", false
		}
	}

	return render(cur), true
}

// render writes a decoded JSON value as plain text.
func render(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case json.Number:
		return x.String()
	case bool:
		return strconv.FormatBool(x)
	default:
		raw, err := json.Marshal(x)
		if err != nil {
			return ""
		}
		return string(raw)
	}
}
