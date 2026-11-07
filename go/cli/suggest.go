package cli

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// notOfferedError means the page a rel was looked for on does not offer it. It exits with
// ExitNotOffered, apart from a usage error: the command line was fine, the name was not there.
type notOfferedError struct{ msg string }

func (e *notOfferedError) Error() string { return e.msg }

func notOfferedf(format string, args ...any) error {
	return &notOfferedError{msg: fmt.Sprintf(format, args...)}
}

// didYouMean is the sentence to add to a not-offered message: a suggestion when exactly one offered
// rel is close to what was typed, and nothing otherwise.
func didYouMean(rel string, offered []string) string {
	if s, ok := suggest(rel, offered); ok {
		return fmt.Sprintf(" Did you mean %q?", s)
	}

	return ""
}

// suggest finds the one offered rel that a mistyped rel was probably meant to be. A rel is close when
// it is two edits or fewer away, or when one contains the other, ignoring case. It suggests nothing
// when none is close, when several are (a guess among several would be a guess), and when the rel is
// already offered, because then it is not a typo. A rel listed twice counts once.
func suggest(rel string, offered []string) (string, bool) {
	want := strings.ToLower(strings.TrimSpace(rel))
	if want == "" {
		return "", false
	}

	seen := map[string]bool{}
	var near []string
	for _, o := range offered {
		lo := strings.ToLower(o)
		if seen[lo] {
			continue
		}
		seen[lo] = true

		switch {
		case lo == want:
			return "", false
		case editDistance(want, lo) <= 2, strings.Contains(lo, want), strings.Contains(want, lo):
			near = append(near, o)
		}
	}
	if len(near) == 1 {
		return near[0], true
	}

	return "", false
}

// editDistance is the number of single-letter insertions, deletions and changes that turn a into b.
func editDistance(a, b string) int {
	x, y := []rune(a), []rune(b)
	if utf8.RuneCountInString(a) == 0 {
		return len(y)
	}
	prev := make([]int, len(y)+1)
	cur := make([]int, len(y)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(x); i++ {
		cur[0] = i
		for j := 1; j <= len(y); j++ {
			cost := 1
			if x[i-1] == y[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}

	return prev[len(y)]
}
