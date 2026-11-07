package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/lukepuplett/surfdude/go/hypertext"
	"github.com/lukepuplett/surfdude/go/surf"
)

// show and explore are the first two commands, kept as they were: same output, and exit 1 for any
// failure, 0 for success, and for explore 1 when it finds something wrong.

func show(ctx context.Context, w io.Writer, s *surf.Surfer) error {
	page, err := s.Root(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "%s  %d  read by the %s dialect\n", page.URL, page.Status, page.Dialect)
	for _, n := range page.Notes() {
		fmt.Fprintf(w, "  note: %s\n", n)
	}
	printControls(w, "", page.Resource.Controls)
	for i, it := range page.Resource.Items {
		fmt.Fprintf(w, "  item %d\n", i)
		printControls(w, "  ", it.Controls)
	}

	return nil
}

func printControls(w io.Writer, indent string, controls []hypertext.Control) {
	for _, c := range controls {
		kind := "link  "
		if c.Form {
			kind = "form  "
		}
		line := fmt.Sprintf("%s  %s %-6s %-22s %s", indent, kind, c.HTTPMethod(), c.Rel, c.Href)
		var requires []string
		if _, err := c.DecodeExtension("requires", &requires); err == nil && len(requires) > 0 {
			line += "  [needs " + strings.Join(requires, ", ") + "]"
		}
		fmt.Fprintln(w, line)
		for _, in := range c.Inputs {
			req := ""
			if in.Required {
				req = " (required)"
			}
			fmt.Fprintf(w, "%s        - %s%s\n", indent, in.Name, req)
		}
	}
}

func explore(ctx context.Context, w io.Writer, s *surf.Surfer, knownFile string, max int) (int, error) {
	opts := surf.ExploreOptions{MaxPages: max}
	if knownFile != "" {
		known, err := readRels(knownFile)
		if err != nil {
			return 0, err
		}
		opts.KnownRel = func(rel string) bool { return known[rel] }
	}

	report, err := s.Explore(ctx, opts)
	if err != nil {
		return 0, err
	}

	fmt.Fprintf(w, "Reached %d page(s) from the front door by following links:\n", len(report.Visits))
	for _, v := range report.Visits {
		fmt.Fprintf(w, "  %d  %-40s via %-16s offers: %s\n", v.Status, v.URL, v.Via, strings.Join(v.Rels, ", "))
	}
	if len(report.Skipped) > 0 {
		fmt.Fprintf(w, "\nNot followed (%d):\n", len(report.Skipped))
		for _, s := range report.Skipped {
			fmt.Fprintf(w, "  %s\n", s)
		}
	}
	if len(report.Findings) > 0 {
		fmt.Fprintf(w, "\nProblems (%d):\n", len(report.Findings))
		for _, f := range report.Findings {
			fmt.Fprintf(w, "  [%s] %s: %s\n", f.Kind, f.Where, f.Message)
		}
		return 1, nil
	}
	fmt.Fprintln(w, "\nNo problems found.")

	return 0, nil
}

var relRow = regexp.MustCompile("^\\|\\s*`([a-z0-9][a-z0-9-]*)`")

func readRels(path string) (map[string]bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Unable to read the rel registry. %w", err)
	}
	known := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		if m := relRow.FindStringSubmatch(line); m != nil {
			known[m[1]] = true
		} else if t := strings.TrimSpace(line); t != "" && !strings.ContainsAny(t, "|# ") {
			known[t] = true
		}
	}

	return known, nil
}
