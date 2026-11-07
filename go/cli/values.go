package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// usageError is a mistake in how the command was typed. It exits with ExitUsage.
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func usagef(format string, args ...any) error { return &usageError{msg: fmt.Sprintf(format, args...)} }

type argKind int

const (
	kindText  argKind = iota // name=text
	kindJSON                 // name:=JSON
	kindFile                 // name=@path
	kindStdin                // name=@-
	kindEnv                  // name=env:VAR
)

func (k argKind) String() string {
	return [...]string{"text", "JSON", "file", "stdin", "environment"}[k]
}

// arg is one name=value as typed, before the value is read from wherever it comes from.
type arg struct {
	Name string
	Kind argKind
	Raw  string // the text after = or :=, without the @ or env: marker
}

// value is an arg once read.
type value struct {
	Name string
	// V is a string, or the decoded JSON for name:=JSON, where a number is a json.Number so its
	// digits are sent exactly as typed.
	V any
	// Secret means the value must never be printed: it came from stdin, the environment or --secret,
	// or its name says it is a secret, or it was built from a value that was.
	Secret bool
	// Template is true for name=text, the only kind in which {name} is replaced by a kept value.
	Template bool
	// File is the path a name=@file value came from, so --dry-run can say where it came from
	// without printing what could be a long document.
	File string
}

var argName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)

// parseArg reads one name=value token. Only the first = splits it, and a colon before it (name:=)
// means the value is JSON. A value that starts with @ is a file, @- is standard input, and one that
// starts with env: is an environment variable. To send text that starts with one of those, write it
// as JSON: name:='"@text"'.
func parseArg(tok string) (arg, error) {
	i := strings.Index(tok, "=")
	if i < 0 {
		return arg{}, usagef("%q is not name=value. Flags go before the root, and --keep and --secret go inside a step.", tok)
	}

	a := arg{Name: tok[:i], Raw: tok[i+1:]}
	if strings.HasSuffix(a.Name, ":") {
		a.Name, a.Kind = strings.TrimSuffix(a.Name, ":"), kindJSON
	}
	if !argName.MatchString(a.Name) {
		return arg{}, usagef("%q is not a field name. Use letters, digits, _, - and . (a dot nests, as in location.lat).", a.Name)
	}

	if a.Kind == kindText {
		switch {
		case a.Raw == "@-":
			a.Kind, a.Raw = kindStdin, ""
		case strings.HasPrefix(a.Raw, "@") && len(a.Raw) > 1:
			a.Kind, a.Raw = kindFile, a.Raw[1:]
		case strings.HasPrefix(a.Raw, "env:"):
			if a.Raw == "env:" {
				return arg{}, usagef("The value for %s says env: but names no environment variable.", a.Name)
			}
			a.Kind, a.Raw = kindEnv, strings.TrimPrefix(a.Raw, "env:")
		}
	}

	return a, nil
}

// resolver reads values from the places they can come from. Standard input is shared, so it can be
// read only once.
type resolver struct {
	env       func(string) string
	stdin     *bufio.Reader
	stdinUsed bool
}

// resolve reads one arg's value.
func (r *resolver) resolve(a arg) (value, error) {
	v := value{Name: a.Name, Secret: isSecretName(a.Name)}

	switch a.Kind {
	case kindText:
		v.V, v.Template = a.Raw, true

	case kindJSON:
		dec := json.NewDecoder(strings.NewReader(a.Raw))
		dec.UseNumber()
		var decoded any
		if err := dec.Decode(&decoded); err != nil {
			return value{}, usagef("The value for %s is not valid JSON. Write text as %s=text, and JSON as %s:=VALUE.", a.Name, a.Name, a.Name)
		}
		if _, err := dec.Token(); err != io.EOF {
			return value{}, usagef("The value for %s has more after the JSON value. Write one value after :=.", a.Name)
		}
		v.V = decoded

	case kindFile:
		raw, err := os.ReadFile(a.Raw)
		if err != nil {
			return value{}, usagef("Unable to read the file for %s: %v.", a.Name, err)
		}
		v.V, v.File = trimNewline(string(raw)), a.Raw

	case kindStdin:
		text, err := r.readStdin(a.Name)
		if err != nil {
			return value{}, err
		}
		v.V, v.Secret = trimNewline(text), true

	case kindEnv:
		text := r.env(a.Raw)
		if text == "" {
			return value{}, usagef("The environment variable %s, for %s, is not set.", a.Raw, a.Name)
		}
		v.V, v.Secret = text, true
	}

	return v, nil
}

// readStdin reads all of standard input, once.
func (r *resolver) readStdin(name string) (string, error) {
	if r.stdin == nil || r.stdinUsed {
		return "", usagef("Standard input can be read once, and it has already been read, so %s cannot also come from it.", name)
	}
	r.stdinUsed = true
	raw, err := io.ReadAll(r.stdin)
	if err != nil {
		return "", usagef("Unable to read standard input for %s: %v.", name, err)
	}

	return string(raw), nil
}

// readStdinLine reads one line of standard input, for a secret that is not typed at a terminal.
func (r *resolver) readStdinLine(name string) (string, error) {
	if r.stdin == nil {
		return "", usagef("There is no standard input to read %s from.", name)
	}
	r.stdinUsed = true
	line, err := r.stdin.ReadString('\n')
	if err != nil && (err != io.EOF || line == "") {
		return "", usagef("Standard input gave nothing for %s. Pipe the secret in, one line per --secret.", name)
	}

	return trimNewline(line), nil
}

// trimNewline removes one trailing newline, so a secret piped in with echo has no stray line ending.
func trimNewline(s string) string {
	s = strings.TrimSuffix(s, "\n")
	return strings.TrimSuffix(s, "\r")
}

var secretWords = [][]byte{[]byte("credential"), []byte("token"), []byte("password"), []byte("secret")}

// isSecretName says whether a field name looks like it holds a secret, so its value is hidden
// wherever the command shows a request. It errs towards hiding: a field called tokenizer is hidden
// too. The agent key is not a secret and is not hidden.
func isSecretName(name string) bool {
	lower := bytes.ToLower([]byte(name))
	for _, w := range secretWords {
		if bytes.Contains(lower, w) {
			return true
		}
	}

	return false
}
