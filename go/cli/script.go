package cli

import (
	"context"
	"os"
	"strings"
)

// runScript runs a journey written in a file. The first line says where to start (root URL), and each
// later line is one step in the chain syntax, so the file is exactly the chain you could have typed,
// with the words then replaced by new lines. Blank lines and lines starting with # are skipped.
// Nothing is sent until the whole file has been read, so a typo on the last line is found before the
// first request, and each error names its line.
func (c *cmd) runScript(ctx context.Context, path string) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		return c.usage(usagef("Unable to read the script: %v.", err))
	}

	var (
		root  string
		steps []step
	)
	for i, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		words, err := splitWords(line)
		if err != nil {
			return c.usage(usagef("%s, line %d: %v", path, i+1, err))
		}

		if root == "" {
			if len(words) != 2 || words[0] != "root" {
				return c.usage(usagef("%s, line %d: the first line must say where to start: root URL.", path, i+1))
			}
			root = words[1]
			continue
		}

		more, err := parseSteps(words)
		if err != nil {
			return c.usage(usagef("%s, line %d: %v", path, i+1, err))
		}
		steps = append(steps, more...)
	}

	switch {
	case root == "":
		return c.usage(usagef("%s has no root line. The first line must say where to start: root URL.", path))
	case len(steps) == 0:
		return c.usage(usagef("%s has no steps after the root line.", path))
	}

	return c.runSteps(ctx, root, steps)
}
