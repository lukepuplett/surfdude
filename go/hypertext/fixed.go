package hypertext

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// ReadOnly says whether the API marked this input "not for editing", with JSON Schema's readOnly:
// true. An input like that is shown and sent, carrying the value the API gave it, but a caller is
// not welcome to change it. Anything else, including a readOnly that is not the boolean true, is an
// ordinary input.
func (i Input) ReadOnly() bool {
	readOnly, _ := i.Schema["readOnly"].(bool)
	return readOnly
}

// queryExpansion is a URI template's query expansion, such as {?to,from}. It is how a template
// declares a parameter the caller may fill, so it is not a fixed value.
var queryExpansion = regexp.MustCompile(`\{[?&][^}]*\}`)

// RepeatedFixedParams names the inputs of the control that repeat a fixed query parameter of its
// href, in input order. A parameter written into the href is a fixed value: the client keeps it and
// sends it unchanged. An input with the same name offers the same value as editable too, so the
// "fixed" value is not fixed. That is a mistake in the API, and the explorer reports it.
//
// Only literal values count. A {template} expansion or a {placeholder} value is how the href says
// "the caller fills this", not a fixed value. An href that cannot be read repeats nothing.
func (c Control) RepeatedFixedParams() []string {
	href := queryExpansion.ReplaceAllString(c.Href, "")
	if _, err := url.Parse(href); err != nil {
		return nil
	}
	_, rawQuery, found := strings.Cut(href, "?")
	if !found || len(c.Inputs) == 0 {
		return nil
	}
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return nil
	}

	fixed := map[string]bool{}
	for name, values := range query {
		for _, v := range values {
			if !strings.Contains(v, "{") {
				fixed[name] = true
			}
		}
	}

	var repeated []string
	for _, in := range c.Inputs {
		if fixed[in.Name] {
			repeated = append(repeated, in.Name)
		}
	}

	return repeated
}

// ReadOnlyInputError means a value was given for an input the form marks read-only, and it is not the
// value the form offered. It names the input and never repeats a value, because the value may be a
// secret and errors reach terminals and logs.
type ReadOnlyInputError struct {
	Rel  string
	Name string
}

func (e *ReadOnlyInputError) Error() string {
	return fmt.Sprintf("Unable to submit %q. The input %s is fixed by the form and is not for editing: leave it out, and it is sent as the form offers it.", e.Rel, e.Name)
}
