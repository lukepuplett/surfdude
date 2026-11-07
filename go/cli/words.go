package cli

import (
	"strings"
	"unicode"
)

// splitWords splits a line into words the way a shell does, for the --sign-in string and for script
// files, which have no shell to do it. Whitespace separates words. Single quotes keep everything
// inside them as it is. Double quotes keep everything too, except that \" and \\ are a quote and a
// backslash. Outside quotes, a backslash makes the next character ordinary. A quote or a backslash
// that is never closed is an error, so a typo does not silently change what is sent.
func splitWords(s string) ([]string, error) {
	var (
		words  []string
		cur    strings.Builder
		inWord bool
		quote  rune
	)
	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case quote == '\'':
			if r == '\'' {
				quote = 0
			} else {
				cur.WriteRune(r)
			}

		case quote == '"':
			switch {
			case r == '"':
				quote = 0
			case r == '\\' && i+1 < len(runes) && (runes[i+1] == '"' || runes[i+1] == '\\'):
				i++
				cur.WriteRune(runes[i])
			default:
				cur.WriteRune(r)
			}

		case r == '\'' || r == '"':
			quote, inWord = r, true

		case r == '\\':
			if i+1 >= len(runes) {
				return nil, usagef("There is a backslash at the end with nothing after it.")
			}
			i++
			cur.WriteRune(runes[i])
			inWord = true

		case unicode.IsSpace(r):
			if inWord {
				words, inWord = append(words, cur.String()), false
				cur.Reset()
			}

		default:
			cur.WriteRune(r)
			inWord = true
		}
	}

	if quote != 0 {
		return nil, usagef("There is a %c quote that is never closed.", quote)
	}
	if inWord {
		words = append(words, cur.String())
	}

	return words, nil
}
