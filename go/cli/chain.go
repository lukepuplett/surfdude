package cli

import (
	"regexp"
	"strconv"
	"strings"
)

// step is one thing to do: follow the control with a rel, sending these values.
type step struct {
	Rel     string
	Args    []arg
	Secrets []string // --secret NAME: prompt for or read these values, never take them from the command line
	Keeps   []keep   // --keep NAME=PATH: capture a value from this step's response
	// Expect is --expect STATUS[,STATUS]: the statuses that count as success for this step, in place of
	// any 2xx. Nil means the default.
	Expect []int
	// Unchecked is --unchecked: send the values as given, without checking them against the form.
	Unchecked bool
}

// keep captures a value from a step's response for later steps to use as {name}. Pattern, when set,
// pulls part of the text out: the first group if the pattern has one, otherwise the whole match.
type keep struct{ Name, Path, Pattern string }

// keptValue is a captured value.
type keptValue struct {
	Text   string
	Secret bool
}

var keepName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

// parseSteps reads a chain: steps joined by the word then. A step is a rel, then name=value fields,
// then --keep and --secret flags in any order. Nothing is sent or read here, so a chain that cannot be
// understood is refused before the first request.
func parseSteps(tokens []string) ([]step, error) {
	if len(tokens) == 0 {
		return nil, usagef("Nothing to do. Say what to do after the root: surf ROOT REL name=value ...")
	}

	var groups [][]string
	var cur []string
	for _, t := range tokens {
		if t == "then" {
			groups, cur = append(groups, cur), nil
			continue
		}
		cur = append(cur, t)
	}
	groups = append(groups, cur)

	steps := make([]step, 0, len(groups))
	for i, g := range groups {
		if len(g) == 0 {
			return nil, usagef("There is no step %d: then needs a step on each side of it.", i+1)
		}
		s, err := parseStep(g)
		if err != nil {
			return nil, usagef("Step %d: %v", i+1, err)
		}
		steps = append(steps, s)
	}

	return steps, nil
}

func parseStep(tokens []string) (step, error) {
	s := step{Rel: tokens[0]}
	if strings.Contains(s.Rel, "=") || strings.HasPrefix(s.Rel, "-") {
		return step{}, usagef("%q is not a rel. A step starts with the name of what to do, such as register.", s.Rel)
	}

	seen := map[string]bool{}
	claim := func(name string) error {
		if seen[name] {
			return usagef("%s is given twice.", name)
		}
		seen[name] = true
		return nil
	}

	for i := 1; i < len(tokens); i++ {
		switch t := tokens[i]; t {
		case "--keep":
			i++
			if i >= len(tokens) {
				return step{}, usagef("--keep needs NAME=PATH, for example --keep code=data.0.text.")
			}
			name, rest, found := strings.Cut(tokens[i], "=")
			path, pattern, hasPattern := strings.Cut(rest, "~")
			if !found || path == "" || !keepName.MatchString(name) || (hasPattern && pattern == "") {
				return step{}, usagef("--keep needs NAME=PATH or NAME=PATH~PATTERN, for example --keep code=data.0.text~[0-9]{6}, not %q.", tokens[i])
			}
			if hasPattern {
				if _, err := regexp.Compile(pattern); err != nil {
					return step{}, usagef("--keep %s: the pattern after ~ is not valid: %v.", name, err)
				}
			}
			s.Keeps = append(s.Keeps, keep{Name: name, Path: path, Pattern: pattern})

		case "--expect":
			i++
			if i >= len(tokens) {
				return step{}, usagef("--expect needs the statuses to expect, for example --expect 429 or --expect 422,429.")
			}
			if err := claim("--expect"); err != nil {
				return step{}, err
			}
			codes, err := parseStatuses(tokens[i])
			if err != nil {
				return step{}, err
			}
			s.Expect = codes

		case "--unchecked":
			if err := claim("--unchecked"); err != nil {
				return step{}, err
			}
			s.Unchecked = true

		case "--secret":
			i++
			if i >= len(tokens) {
				return step{}, usagef("--secret needs the name of a field, for example --secret credential.")
			}
			if !argName.MatchString(tokens[i]) {
				return step{}, usagef("--secret needs the name of a field, not %q.", tokens[i])
			}
			if err := claim(tokens[i]); err != nil {
				return step{}, err
			}
			s.Secrets = append(s.Secrets, tokens[i])

		default:
			if strings.HasPrefix(t, "-") {
				return step{}, usagef("%q is a flag, and flags go before the root. Only --keep, --secret, --expect and --unchecked go inside a step.", t)
			}
			a, err := parseArg(t)
			if err != nil {
				return step{}, err
			}
			if err := claim(a.Name); err != nil {
				return step{}, err
			}
			s.Args = append(s.Args, a)
		}
	}

	return s, nil
}

// parseStatuses reads --expect's value: one or more HTTP statuses from 100 to 599, separated by
// commas, with nothing else in it. A mistake is refused before anything is sent.
func parseStatuses(text string) ([]int, error) {
	bad := func() error {
		return usagef("--expect needs one or more status codes from 100 to 599, separated by commas, such as 429 or 422,429, not %q.", text)
	}

	var codes []int
	for _, part := range strings.Split(text, ",") {
		code, err := strconv.Atoi(part)
		if err != nil || code < 100 || code > 599 {
			return nil, bad()
		}
		codes = append(codes, code)
	}

	return codes, nil
}

// extract pulls part of a text out by a pattern: the first group if the pattern has one, otherwise the
// whole match. The second result is false when nothing matches.
func extract(text, pattern string) (string, bool) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", false
	}
	m := re.FindStringSubmatch(text)
	switch {
	case m == nil:
		return "", false
	case len(m) > 1:
		return m[1], true
	default:
		return m[0], true
	}
}

var templatePart = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_.-]*)\}`)

// templateNames lists the {names} in a text value.
func templateNames(s string) []string {
	var out []string
	for _, m := range templatePart.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

// validateTemplates checks that every {name} a step uses was kept by an earlier step, so a typo is
// found before anything is sent. Only name=text values are templates.
func validateTemplates(steps []step) error {
	kept := map[string]bool{}
	for i, s := range steps {
		for _, a := range s.Args {
			if a.Kind != kindText {
				continue
			}
			for _, name := range templateNames(a.Raw) {
				if !kept[name] {
					return usagef("Step %d (%s) uses {%s}, but no earlier step keeps it. Add --keep %s=PATH to an earlier step.", i+1, s.Rel, name, name)
				}
			}
		}
		for _, k := range s.Keeps {
			kept[k.Name] = true
		}
	}

	return nil
}

// expandTemplate replaces each {name} with the value an earlier step kept. The result is marked
// secret when any value put into it was.
func expandTemplate(s string, kept map[string]keptValue) (string, bool, error) {
	secret := false
	var missing string
	out := templatePart.ReplaceAllStringFunc(s, func(match string) string {
		name := match[1 : len(match)-1]
		v, ok := kept[name]
		if !ok {
			if missing == "" {
				missing = name
			}
			return match
		}
		secret = secret || v.Secret
		return v.Text
	})
	if missing != "" {
		return "", false, usagef("{%s} has no value: no earlier step kept it.", missing)
	}

	return out, secret, nil
}
