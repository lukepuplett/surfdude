package surftest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/lukepuplett/surfdude/go/surf"
)

const (
	// UpdateGoldenEnv, set to anything but empty, makes Golden write the file instead of checking it.
	UpdateGoldenEnv = "SURFDUDE_UPDATE_GOLDEN"

	baseToken   = "{base}"
	maskedToken = "<masked>"
	maxDiffs    = 10
)

// GoldenOption changes how Golden compares a response.
type GoldenOption func(*goldenConfig)

type goldenConfig struct {
	masks []string
}

// Mask replaces the value at each JSON path with "<masked>" before comparing, for fields that change
// every run, such as an expiry time. A path is dotted, with [i] for an array index, for example
// data[0].expiresAt. A path the response does not have is ignored.
func Mask(paths ...string) GoldenOption {
	return func(c *goldenConfig) { c.masks = append(c.masks, paths...) }
}

// Golden checks the response body of a page against a checked-in JSON file, so a documented sample
// is the real response and cannot drift from it.
//
// Before comparing it makes the body repeatable: the address of the server the page came from is
// replaced by {base} (a test server's port changes every run), masked fields are replaced by
// <masked>, and the JSON is indented with the server's own key order kept, so the file reads like
// the response and a diff shows only what changed.
//
// With the environment variable SURFDUDE_UPDATE_GOLDEN set, it writes the file instead. Review the
// diff before committing it: the file is the documented contract.
func Golden(t testing.TB, p *surf.Page, path string, opts ...GoldenOption) {
	t.Helper()

	var cfg goldenConfig
	for _, o := range opts {
		o(&cfg)
	}

	text := string(p.RawBody())
	if p.URL != nil {
		text = strings.ReplaceAll(text, p.URL.Scheme+"://"+p.URL.Host, baseToken)
	}
	root, err := parseOrdered([]byte(text))
	if err != nil {
		t.Fatalf("Unable to use %s as a golden file. The response body is not JSON: %v", path, err)
		return
	}
	for _, m := range cfg.masks {
		root.mask(m)
	}
	got := root.pretty()

	if os.Getenv(UpdateGoldenEnv) != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("Unable to write %s. %v.", path, err)
			return
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("Unable to write %s. %v.", path, err)
			return
		}
		t.Logf("Updated %s. Review the diff before committing it.", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Unable to check %s. %v. Run the test with %s=1 to create it, then review the file.", path, err, UpdateGoldenEnv)
		return
	}
	if string(want) == got {
		return
	}

	diffs := diffJSON(want, []byte(got))
	if len(diffs) == 0 {
		diffs = []string{"the JSON data is the same, but the layout or key order differs"}
	}
	t.Errorf("The response no longer matches %s:\n  %s\nIf the change is right, run the test with %s=1 and review the diff.",
		path, strings.Join(diffs, "\n  "), UpdateGoldenEnv)
}

// node is JSON that remembers the order of an object's keys, which a Go map does not.
type node struct {
	kind  byte // '{' object, '[' array, 'v' scalar
	keys  []string
	items []*node
	raw   []byte // a scalar, already encoded
}

func parseOrdered(data []byte) (*node, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber() // keep numbers exactly as the server wrote them
	n, err := parseValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("there is more than one JSON value")
	}
	return n, nil
}

func parseValue(dec *json.Decoder) (*node, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, isDelim := tok.(json.Delim)
	if !isDelim {
		return &node{kind: 'v', raw: encodeScalar(tok)}, nil
	}

	n := &node{kind: byte(delim)}
	for dec.More() {
		if delim == '{' {
			k, err := dec.Token()
			if err != nil {
				return nil, err
			}
			n.keys = append(n.keys, k.(string))
		}
		child, err := parseValue(dec)
		if err != nil {
			return nil, err
		}
		n.items = append(n.items, child)
	}
	if _, err := dec.Token(); err != nil { // the closing } or ]
		return nil, err
	}
	return n, nil
}

// encodeScalar writes a string, number, bool or null without escaping < > & the way json.Marshal
// would, so the file reads the same as the response.
func encodeScalar(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return bytes.TrimRight(buf.Bytes(), "\n")
}

func (n *node) compact(buf *bytes.Buffer) {
	switch n.kind {
	case 'v':
		buf.Write(n.raw)
	case '{':
		buf.WriteByte('{')
		for i, child := range n.items {
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.Write(encodeScalar(n.keys[i]))
			buf.WriteByte(':')
			child.compact(buf)
		}
		buf.WriteByte('}')
	case '[':
		buf.WriteByte('[')
		for i, child := range n.items {
			if i > 0 {
				buf.WriteByte(',')
			}
			child.compact(buf)
		}
		buf.WriteByte(']')
	}
}

// pretty is the JSON indented by two spaces, ending in a newline.
func (n *node) pretty() string {
	var compact, out bytes.Buffer
	n.compact(&compact)
	if err := json.Indent(&out, compact.Bytes(), "", "  "); err != nil {
		return compact.String() + "\n"
	}
	return out.String() + "\n"
}

var pathPart = regexp.MustCompile(`^([^\[\]]*)((?:\[\d+\])*)$`)
var pathIndex = regexp.MustCompile(`\[(\d+)\]`)

// mask replaces the value at a dotted path, such as data[0].expiresAt. A path that is not there is
// ignored.
func (n *node) mask(path string) {
	cur := n
	for _, part := range strings.Split(strings.TrimPrefix(strings.TrimPrefix(path, "$"), "."), ".") {
		m := pathPart.FindStringSubmatch(part)
		if m == nil {
			return
		}
		if m[1] != "" {
			if cur.kind != '{' {
				return
			}
			next := (*node)(nil)
			for i, k := range cur.keys {
				if k == m[1] {
					next = cur.items[i]
					break
				}
			}
			if next == nil {
				return
			}
			cur = next
		}
		for _, idx := range pathIndex.FindAllStringSubmatch(m[2], -1) {
			i, _ := strconv.Atoi(idx[1])
			if cur.kind != '[' || i >= len(cur.items) {
				return
			}
			cur = cur.items[i]
		}
	}
	*cur = node{kind: 'v', raw: encodeScalar(maskedToken)}
}

// diffJSON lists where two JSON documents differ in data, ignoring layout and key order.
func diffJSON(want, got []byte) []string {
	var w, g any
	for _, in := range []struct {
		data []byte
		into *any
	}{{want, &w}, {got, &g}} {
		dec := json.NewDecoder(bytes.NewReader(in.data))
		dec.UseNumber()
		if err := dec.Decode(in.into); err != nil {
			return []string{"the golden file is not valid JSON: " + err.Error()}
		}
	}

	var diffs []string
	walkDiff("", w, g, &diffs)
	if len(diffs) > maxDiffs {
		more := len(diffs) - maxDiffs
		diffs = append(diffs[:maxDiffs], fmt.Sprintf("… and %d more", more))
	}
	return diffs
}

func walkDiff(path string, want, got any, out *[]string) {
	here := path
	if here == "" {
		here = "the document"
	}
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			*out = append(*out, fmt.Sprintf("%s: golden has an object, the response does not", here))
			return
		}
		keys := map[string]bool{}
		for k := range w {
			keys[k] = true
		}
		for k := range g {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			child := k
			if path != "" {
				child = path + "." + k
			}
			wv, inWant := w[k]
			gv, inGot := g[k]
			switch {
			case !inWant:
				*out = append(*out, child+": new in the response")
			case !inGot:
				*out = append(*out, child+": gone from the response")
			default:
				walkDiff(child, wv, gv, out)
			}
		}
	case []any:
		g, ok := got.([]any)
		if !ok || len(w) != len(g) {
			*out = append(*out, fmt.Sprintf("%s: golden has %d items, the response has %d", here, len(w), arrayLen(got)))
			return
		}
		for i := range w {
			walkDiff(fmt.Sprintf("%s[%d]", path, i), w[i], g[i], out)
		}
	default:
		if !reflect.DeepEqual(want, got) {
			*out = append(*out, fmt.Sprintf("%s: golden %v, response %v", here, want, got))
		}
	}
}

func arrayLen(v any) int {
	if a, ok := v.([]any); ok {
		return len(a)
	}
	return -1
}
