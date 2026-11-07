package cli

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/lukepuplett/surfdude/go/hypertext"
	"github.com/lukepuplett/surfdude/go/surf"
)

// printPage writes a response the way a person wants to read it: the status and message, the
// server's notes and problems, a one-line summary of the data, and what can be done next.
//
// The data summary hides the value of any field named like a secret (credential, token, password),
// because a terminal is read over shoulders and kept in scrollback. --json and -q show the real
// value, which is how a script, or a person who means to, reads it.
func printPage(w io.Writer, page *surf.Page) {
	res := page.Resource
	message := res.Message
	if message == "" {
		message = http.StatusText(page.Status)
	}
	fmt.Fprintf(w, "%d %s\n", page.Status, message)
	for _, n := range res.Notes {
		fmt.Fprintf(w, "  note: %s\n", n)
	}
	for _, p := range res.Problems {
		fmt.Fprintf(w, "  problem: %s\n", strings.TrimSpace(strings.Join(nonEmpty(p.Field+":", p.Message), " ")))
	}
	if summary := dataSummary(res); summary != "" {
		fmt.Fprintf(w, "  data: %s\n", summary)
	}
	if len(res.Controls) > 0 {
		fmt.Fprintln(w, "next:")
		for _, c := range res.Controls {
			fmt.Fprintf(w, "  %-18s %-6s %s\n", c.Rel, c.HTTPMethod(), fieldList(c))
		}
	}
}

func nonEmpty(parts ...string) []string {
	var out []string
	for _, p := range parts {
		if p != "" && p != ":" {
			out = append(out, p)
		}
	}
	return out
}

// fieldList names a control's fields, a star after each required one.
func fieldList(c hypertext.Control) string {
	var names []string
	for _, in := range c.Inputs {
		name := in.Name
		if in.Required {
			name += "*"
		}
		names = append(names, name)
	}

	return strings.Join(names, " ")
}

const (
	maxSummaryValue = 40
	maxSummaryRaw   = 80
)

// dataSummary is one line about the first item of data, such as: agentKey=agt_key token=<secret>.
func dataSummary(res *hypertext.Resource) string {
	if len(res.Items) == 0 {
		return ""
	}
	var fields map[string]any
	if err := res.Items[0].Decode(&fields); err != nil || fields == nil {
		return truncate(string(res.Items[0].Data), maxSummaryRaw)
	}

	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+summaryValue(k, fields[k]))
	}
	line := strings.Join(parts, " ")
	if more := len(res.Items) - 1; more > 0 {
		line += fmt.Sprintf("  (+%d more items)", more)
	}

	return line
}

func summaryValue(name string, v any) string {
	if isSecretName(name) {
		return "<secret>"
	}
	switch x := v.(type) {
	case string:
		return truncate(x, maxSummaryValue)
	case map[string]any:
		return "{…}"
	case []any:
		return fmt.Sprintf("[%d]", len(x))
	case nil:
		return "null"
	default:
		return fmt.Sprint(x)
	}
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}

	return s
}

// exitFor is the exit code for an error from a step: 1 when the server refused the request (4xx), 4 when
// it failed (5xx), 2 when the command was wrong, and 3 for anything else, which is a network
// failure or a response the command could not read.
func exitFor(err error) int {
	var status *hypertext.UnexpectedStatusError
	if errors.As(err, &status) {
		if status.Status >= 500 {
			return ExitServer
		}
		return ExitRefused
	}
	if isUsage(err) {
		return ExitUsage
	}

	return ExitNetwork
}

func isUsage(err error) bool {
	var (
		usage      *usageError
		noRel      *hypertext.RelNotFoundError
		missing    *hypertext.MissingInputError
		invalid    *hypertext.InvalidInputError
		unexpected *hypertext.UnexpectedInputError
	)

	return errors.As(err, &usage) || errors.As(err, &noRel) || errors.As(err, &missing) ||
		errors.As(err, &invalid) || errors.As(err, &unexpected)
}

// withHint adds the one piece of advice a typed value usually needs: numbers and booleans are written
// with := so they are sent as numbers and booleans, not as text.
func withHint(err error) error {
	var invalid *hypertext.InvalidInputError
	if errors.As(err, &invalid) {
		for _, kind := range []string{"whole number", "a number", "true or false"} {
			if strings.Contains(invalid.Reason, kind) {
				return fmt.Errorf("%w Write numbers and booleans as %s:=VALUE, for example %s:=3.", err, invalid.Name, invalid.Name)
			}
		}
	}

	return err
}
