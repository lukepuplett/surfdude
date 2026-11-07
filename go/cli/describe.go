package cli

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/lukepuplett/surfdude/go/hypertext"
)

// describe prints what the front door's control with a rel takes, as a fill-in template: each field
// with whether it is required and what the server says is allowed, then a line to start from. It reads
// the front door and sends nothing else.
func (c *cmd) describe(ctx context.Context, rootURL, rel string) int {
	s, err := c.newSurfer(rootURL)
	if err != nil {
		return c.usage(err)
	}
	root, err := s.Root(ctx)
	if err != nil {
		return c.fail(root, err)
	}
	ctrl, ok := root.Control(rel)
	if !ok {
		return c.reject(notOfferedf("Unable to describe %q. The front door offers: %s.%s", rel, listOrNothing(root.Rels()), didYouMean(rel, root.Rels())))
	}
	// Show where the control goes, not the path it was written with.
	if strings.HasPrefix(ctrl.Href, "/") {
		ctrl.Href = root.URL.Scheme + "://" + root.URL.Host + ctrl.Href
	}
	describeControl(c.std.Stdout, rootURL, ctrl, c.masking())

	return ExitOK
}

var hrefVar = regexp.MustCompile(`\{\??([^}]+)\}`)

// hrefVars lists the variables in a URI template: {name} and {?a,b}. They are fields a step can
// fill, though the form does not list them.
func hrefVars(href string) []string {
	var out []string
	for _, m := range hrefVar.FindAllStringSubmatch(href, -1) {
		for _, name := range strings.Split(m[1], ",") {
			out = append(out, strings.TrimSpace(name))
		}
	}

	return out
}

// describeControl prints a control's fields and a line to start from. root is how the front door was
// typed, so the line can be copied. A prefilled value under a hidden name is not shown, unless
// --reveal was given.
func describeControl(w io.Writer, root string, c hypertext.Control, m masking) {
	fmt.Fprintf(w, "%s  %s  %s\n", c.Rel, c.HTTPMethod(), c.Href)

	type field struct {
		name, detail string
		required     bool
		token        string
	}
	var fields []field
	listed := map[string]bool{}
	for _, in := range c.Inputs {
		listed[in.Name] = true
		fields = append(fields, field{
			name:     in.Name,
			detail:   inputDetail(in, m),
			required: in.Required && in.Value == nil,
			token:    fieldToken(in.Name, in.Schema),
		})
	}
	for _, name := range hrefVars(c.Href) {
		if !listed[name] {
			listed[name] = true
			fields = append(fields, field{name: name, detail: "goes into the address", token: name + "=TEXT"})
		}
	}

	if len(fields) == 0 {
		fmt.Fprintln(w, "fields: none")
	} else {
		fmt.Fprintln(w, "fields (a star marks one that is required):")
		label := func(f field) string {
			if f.required {
				return f.name + "*"
			}
			return f.name
		}
		width := 0
		for _, f := range fields {
			width = max(width, len(label(f)))
		}
		for _, f := range fields {
			fmt.Fprintf(w, "  %-*s  %s\n", width, label(f), f.detail)
		}
	}

	line := "surf " + root + " " + c.Rel
	var optional []string
	for _, f := range fields {
		if f.required {
			line += " " + f.token
		} else {
			optional = append(optional, "["+f.token+"]")
		}
	}
	if len(optional) > 0 {
		line += " " + strings.Join(optional, " ")
	}
	fmt.Fprintln(w, "try it:")
	fmt.Fprintf(w, "  %s\n", line)
}

// fieldToken is how a field is written on the command line, by its type: numbers and booleans need :=.
func fieldToken(name string, schema hypertext.Schema) string {
	switch schema["type"] {
	case "integer", "number", "boolean":
		return name + ":=VALUE"
	case "object", "array":
		return name + ":=JSON"
	default:
		return name + "=TEXT"
	}
}

// inputDetail is the type and limits of an input, in a line.
func inputDetail(in hypertext.Input, m masking) string {
	var parts []string
	if t, ok := in.Schema["type"].(string); ok {
		parts = append(parts, t)
	} else {
		parts = append(parts, "any")
	}
	if f, ok := in.Schema["format"].(string); ok {
		parts = append(parts, "format "+f)
	}
	if enum, ok := in.Schema["enum"].([]any); ok && len(enum) > 0 {
		names := make([]string, len(enum))
		for i, e := range enum {
			names[i] = fmt.Sprint(e)
		}
		parts = append(parts, "one of "+strings.Join(names, ", "))
	}
	for _, limit := range []struct{ key, label, unit string }{
		{"minimum", "min", ""}, {"maximum", "max", ""}, {"minLength", "min", " chars"}, {"maxLength", "max", " chars"},
	} {
		if v, ok := in.Schema[limit.key]; ok {
			parts = append(parts, fmt.Sprintf("%s %v%s", limit.label, v, limit.unit))
		}
	}
	if p, ok := in.Schema["pattern"].(string); ok {
		parts = append(parts, "pattern "+p)
	}

	detail := strings.Join(parts, ", ")
	if d, ok := in.Schema["description"].(string); ok && d != "" {
		detail += " - " + d
	}
	if in.Value != nil {
		shown := fmt.Sprint(in.Value)
		if m.hides(in.Name) {
			shown = secretMask
		}
		detail += " (prefilled: " + shown + ")"
	}

	return detail
}
