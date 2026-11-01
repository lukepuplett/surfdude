// Command surf drives a hypermedia API from its front door.
//
//	surf show    http://localhost:8080      print what the front door offers
//	surf explore http://localhost:8080      crawl everything reachable and report problems
//
// Flags:
//
//	-dialects agenttrader,simple   which dialects to understand, in order
//	-bearer TOKEN                  send Authorization: Bearer TOKEN
//	-known-rels FILE               a registry of rels (a markdown table of `rel`s, or one per line)
//	-max N                         stop exploring after N pages
//
// Exit status is 1 if exploring finds anything wrong.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/lukepuplett/surfdude/go/dialect"
	"github.com/lukepuplett/surfdude/go/dialects/agenttrader"
	"github.com/lukepuplett/surfdude/go/dialects/simple"
	"github.com/lukepuplett/surfdude/go/hypertext"
	"github.com/lukepuplett/surfdude/go/surf"
)

func main() {
	dialects := flag.String("dialects", "agenttrader,simple", "dialects to understand, in order")
	bearer := flag.String("bearer", "", "access token to send")
	knownRels := flag.String("known-rels", "", "file listing the rels the API is allowed to use")
	max := flag.Int("max", 200, "stop exploring after this many pages")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: surf [flags] show|explore URL")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}
	command, root := flag.Arg(0), flag.Arg(1)

	var ds []dialect.Dialect
	for _, name := range strings.Split(*dialects, ",") {
		switch strings.TrimSpace(name) {
		case "agenttrader":
			ds = append(ds, agenttrader.New())
		case "simple":
			ds = append(ds, simple.New())
		case "":
		default:
			fail("Unable to start. There is no dialect called %q. Choose from agenttrader, simple.", name)
		}
	}
	opts := []surf.Option{surf.WithDialects(ds...)}
	if *bearer != "" {
		opts = append(opts, surf.WithBearer(*bearer))
	}
	s, err := surf.New(root, opts...)
	if err != nil {
		fail("%v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	switch command {
	case "show":
		show(ctx, s)
	case "explore":
		os.Exit(explore(ctx, s, *knownRels, *max))
	default:
		fail("Unable to run. There is no command %q. Use show or explore.", command)
	}
}

func show(ctx context.Context, s *surf.Surfer) {
	page, err := s.Root(ctx)
	if err != nil {
		fail("%v", err)
	}
	fmt.Printf("%s  %d  read by the %s dialect\n", page.URL, page.Status, page.Dialect)
	for _, n := range page.Notes() {
		fmt.Printf("  note: %s\n", n)
	}
	printControls("", page.Resource.Controls)
	for i, it := range page.Resource.Items {
		fmt.Printf("  item %d\n", i)
		printControls("  ", it.Controls)
	}
}

func printControls(indent string, controls []hypertext.Control) {
	for _, c := range controls {
		kind := "link  "
		if c.Form {
			kind = "form  "
		}
		line := fmt.Sprintf("%s  %s %-6s %-22s %s", indent, kind, c.HTTPMethod(), c.Rel, c.Href)
		if len(c.Requires) > 0 {
			line += "  [needs " + strings.Join(c.Requires, ", ") + "]"
		}
		fmt.Println(line)
		for _, in := range c.Inputs {
			req := ""
			if in.Required {
				req = " (required)"
			}
			fmt.Printf("%s        - %s%s\n", indent, in.Name, req)
		}
	}
}

func explore(ctx context.Context, s *surf.Surfer, knownFile string, max int) int {
	opts := surf.ExploreOptions{MaxPages: max}
	if knownFile != "" {
		known, err := readRels(knownFile)
		if err != nil {
			fail("%v", err)
		}
		opts.KnownRel = func(rel string) bool { return known[rel] }
	}

	report, err := s.Explore(ctx, opts)
	if err != nil {
		fail("%v", err)
	}

	fmt.Printf("Reached %d page(s) from the front door by following links:\n", len(report.Visits))
	for _, v := range report.Visits {
		fmt.Printf("  %d  %-40s via %-16s offers: %s\n", v.Status, v.URL, v.Via, strings.Join(v.Rels, ", "))
	}
	if len(report.Skipped) > 0 {
		fmt.Printf("\nNot followed (%d):\n", len(report.Skipped))
		for _, s := range report.Skipped {
			fmt.Printf("  %s\n", s)
		}
	}
	if len(report.Findings) > 0 {
		fmt.Printf("\nProblems (%d):\n", len(report.Findings))
		for _, f := range report.Findings {
			fmt.Printf("  [%s] %s: %s\n", f.Kind, f.Where, f.Message)
		}
		return 1
	}
	fmt.Println("\nNo problems found.")
	return 0
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

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
