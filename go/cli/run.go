// Package cli is the surf command line, written as a function so a test can call it with its own
// arguments, input, output and environment and no subprocess. cmd/surf is a few lines that call Run.
//
// The commands: show and explore read an API; surf ROOT REL name=value ... takes one step; steps joined
// by the word then are a chain. A step names only a rel, never a URL or a method: the front door
// says what is possible and the command follows it, like a journey. See docs/cli.md.
package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/lukepuplett/surfdude/go/dialect"
	"github.com/lukepuplett/surfdude/go/dialects/agenttrader"
	"github.com/lukepuplett/surfdude/go/dialects/simple"
	"github.com/lukepuplett/surfdude/go/surf"
)

// Exit codes. They are part of the command's contract, so a script can tell what went wrong.
const (
	ExitOK      = 0 // it worked
	ExitRefused = 1 // the server refused the request (4xx), or explore found a problem
	ExitUsage   = 2 // the command was wrong: nothing was sent
	ExitNetwork = 3 // the server could not be reached, or its answer could not be read
	ExitServer  = 4 // the server failed (5xx)
)

// IO is what the command talks to.
type IO struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	// StdinIsTerminal is true when Stdin is a person at a keyboard.
	StdinIsTerminal bool
	// ReadSecret prompts and reads one line with the typing hidden. Nil means this platform cannot.
	ReadSecret func(prompt string) (string, error)
	// Client is the HTTP client. Nil means a default one with a timeout.
	Client *http.Client
}

const usageText = `usage: surf [flags] ROOT                       show what the front door offers
       surf [flags] ROOT REL [name=value ...]  take one step: find REL on the page and send it
       surf [flags] ROOT REL ... then REL ...  take several steps, one after another
       surf [flags] describe ROOT REL          what does this form take?
       surf [flags] run FILE                   run a journey written in a file: root URL, then a step a line
       surf [flags] show|explore ROOT          read an API, or crawl it and report problems

values:  name=text   name:=JSON   name=@file   name=@-   name=env:VAR
flags go before ROOT:`

type flags struct {
	dialects  string
	bearer    string
	bearerEnv string
	knownRels string
	query     string
	max       int
	json      bool
	dryRun    bool
	signIn    string
	tokenPath string
}

// cmd is one run of the command.
type cmd struct {
	std IO
	env func(string) string
	f   flags
}

// Run runs the command and returns its exit code. Everything it prints goes to std.
func Run(args []string, std IO, env func(string) string) int {
	if std.Stdin == nil {
		std.Stdin = strings.NewReader("")
	}
	c := &cmd{std: std, env: env}

	fs := flag.NewFlagSet("surf", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&c.f.dialects, "dialects", "agenttrader,simple", "dialects to understand, in order")
	fs.StringVar(&c.f.bearer, "bearer", "", "access token to send")
	fs.StringVar(&c.f.bearerEnv, "bearer-env", "", "environment variable that holds the access token to send")
	fs.StringVar(&c.f.knownRels, "known-rels", "", "file listing the rels the API is allowed to use (explore)")
	fs.StringVar(&c.f.query, "q", "", "print one value from the response, by dotted path such as data.0.token")
	fs.IntVar(&c.f.max, "max", 200, "stop exploring after this many pages")
	fs.BoolVar(&c.f.json, "json", false, "print the whole response as JSON")
	fs.BoolVar(&c.f.dryRun, "dry-run", false, "show the request the first step would send, with secrets hidden, and send nothing")
	fs.StringVar(&c.f.signIn, "sign-in", "", "values for the sign-in rel, such as 'credential=env:CRED': on a 401, sign in with them and try the step again once")
	fs.StringVar(&c.f.tokenPath, "token-path", "data.0.token", "where the access token is in the sign-in response")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			c.printUsage(std.Stdout, fs)
			return ExitOK
		}
		fmt.Fprintf(std.Stderr, "Unable to run. %v.\n", err)
		c.printUsage(std.Stderr, fs)
		return ExitUsage
	}
	rest := fs.Args()
	if len(rest) == 0 {
		c.printUsage(std.Stderr, fs)
		return ExitUsage
	}
	if c.f.json && c.f.query != "" {
		return c.usage(usagef("Use --json or -q, not both: --json prints everything and -q prints one value."))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	switch rest[0] {
	case "help":
		c.printUsage(std.Stdout, fs)
		return ExitOK
	case "show", "explore":
		if len(rest) != 2 {
			c.printUsage(std.Stderr, fs)
			return ExitUsage
		}
		return c.legacy(ctx, rest[0], rest[1])
	case "describe":
		if len(rest) != 3 {
			return c.usage(usagef("usage: surf describe ROOT REL"))
		}
		return c.describe(ctx, rest[1], rest[2])
	case "run":
		if len(rest) != 2 {
			return c.usage(usagef("usage: surf run FILE"))
		}
		return c.runScript(ctx, rest[1])
	}

	if len(rest) == 1 { // a root alone shows the front door
		return c.legacy(ctx, "show", rest[0])
	}
	steps, err := parseSteps(rest[1:])
	if err != nil {
		return c.usage(err)
	}

	return c.runSteps(ctx, rest[0], steps)
}

func (c *cmd) printUsage(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprintln(w, usageText)
	fs.SetOutput(w)
	fs.PrintDefaults()
	fs.SetOutput(io.Discard)
}

// usage prints a mistake in the command and returns the usage exit code.
func (c *cmd) usage(err error) int {
	fmt.Fprintln(c.std.Stderr, err)
	return ExitUsage
}

// parseDialects turns the --dialects flag into dialects.
func (c *cmd) parseDialects() ([]dialect.Dialect, error) {
	var ds []dialect.Dialect
	for _, name := range strings.Split(c.f.dialects, ",") {
		switch strings.TrimSpace(name) {
		case "agenttrader":
			ds = append(ds, agenttrader.New())
		case "simple":
			ds = append(ds, simple.New())
		case "":
		default:
			return nil, usagef("Unable to start. There is no dialect called %q. Choose from agenttrader, simple.", name)
		}
	}

	return ds, nil
}

// token is the bearer token from --bearer or --bearer-env, or empty.
func (c *cmd) token() (string, error) {
	switch {
	case c.f.bearer != "" && c.f.bearerEnv != "":
		return "", usagef("Use --bearer or --bearer-env, not both.")
	case c.f.bearerEnv != "":
		token := c.env(c.f.bearerEnv)
		if token == "" {
			return "", usagef("The environment variable %s, named by --bearer-env, is not set.", c.f.bearerEnv)
		}
		return token, nil
	}

	return c.f.bearer, nil
}

// newSurfer builds the surfer for a root, with the dialects, the token and any extra options.
func (c *cmd) newSurfer(root string, extra ...surf.Option) (*surf.Surfer, error) {
	ds, err := c.parseDialects()
	if err != nil {
		return nil, err
	}
	token, err := c.token()
	if err != nil {
		return nil, err
	}

	opts := append([]surf.Option{surf.WithDialects(ds...)}, extra...)
	if token != "" {
		opts = append(opts, surf.WithBearer(token))
	}
	if c.std.Client != nil {
		opts = append(opts, surf.WithHTTPClient(c.std.Client))
	}
	s, err := surf.New(normaliseRoot(root), opts...)
	if err != nil {
		return nil, usagef("%v", err)
	}

	return s, nil
}

// normaliseRoot lets a root be typed without https://. A host on this machine gets http://, because it
// is not behind TLS.
func normaliseRoot(s string) string {
	if strings.Contains(s, "://") {
		return s
	}
	host, _, _ := strings.Cut(s, "/")
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "localhost" || host == "::1" || strings.HasPrefix(host, "127.") {
		return "http://" + s
	}

	return "https://" + s
}

// legacy runs show and explore exactly as they always ran: the same output, 0 on success and 1 on
// any failure.
func (c *cmd) legacy(ctx context.Context, command, root string) int {
	s, err := c.newSurfer(root)
	if err != nil {
		fmt.Fprintln(c.std.Stderr, err)
		return 1
	}

	switch command {
	case "explore":
		code, err := explore(ctx, c.std.Stdout, s, c.f.knownRels, c.f.max)
		if err != nil {
			fmt.Fprintln(c.std.Stderr, err)
			return 1
		}
		return code
	default:
		if err := show(ctx, c.std.Stdout, s); err != nil {
			fmt.Fprintln(c.std.Stderr, err)
			return 1
		}
		return 0
	}
}

// runSteps takes the steps one after another. Nothing is sent until every step has been read and
// every value found, so a mistake anywhere in the chain is refused before the first request.
func (c *cmd) runSteps(ctx context.Context, rootURL string, steps []step) int {
	if err := validateTemplates(steps); err != nil {
		return c.usage(err)
	}

	r := &resolver{env: c.env, stdin: bufio.NewReader(c.std.Stdin)}
	resolved := make([][]value, len(steps))
	for i, st := range steps {
		vals, err := c.resolveStep(r, st)
		if err != nil {
			return c.usage(err)
		}
		resolved[i] = vals
	}

	signInValues, err := c.resolveSignIn(r)
	if err != nil {
		return c.usage(err)
	}
	var rootPage *surf.Page
	var extra []surf.Option
	if signInValues != nil {
		extra = append(extra, surf.WithSignIn(c.signInHook(signInValues, &rootPage)))
	}

	s, err := c.newSurfer(rootURL, extra...)
	if err != nil {
		return c.usage(err)
	}
	root, err := s.Root(ctx)
	if err != nil {
		return c.fail(root, err)
	}
	rootPage = root
	if c.f.dryRun {
		return c.dryRun(ctx, root, steps, resolved)
	}

	cur, last := root, root
	kept := map[string]keptValue{}
	for i, st := range steps {
		page, err := pickPage(cur, root, st.Rel)
		if err != nil {
			return c.usage(err)
		}
		vals, mask, err := buildValues(resolved[i], kept)
		if err != nil {
			return c.usage(err)
		}

		req, err := page.Prepare(ctx, st.Rel, surf.With(vals))
		if err != nil {
			return c.usage(withHint(err))
		}
		c.announce(req, st.Rel, mask)

		resp, err := page.Submit(ctx, st.Rel, vals)
		if err != nil {
			return c.fail(resp, err)
		}
		for _, k := range st.Keeps {
			text, ok := lookup(resp.RawBody(), k.Path)
			if !ok {
				return c.usage(usagef("Unable to keep %s: the response has no value at %s.", k.Name, k.Path))
			}
			if k.Pattern != "" {
				if text, ok = extract(text, k.Pattern); !ok {
					return c.usage(usagef("Unable to keep %s: the value at %s does not match the pattern %s.", k.Name, k.Path, k.Pattern))
				}
			}
			kept[k.Name] = keptValue{Text: text, Secret: isSecretName(k.Name) || isSecretName(k.Path)}
		}
		cur, last = resp, resp
	}

	return c.finish(last)
}

// announce says, on stderr and before sending, what a step that changes something is about to do. A
// read says nothing. The address is masked, so a secret in a query string is not shown.
func (c *cmd) announce(req *http.Request, rel string, mask map[string]string) {
	if req.Method != http.MethodGet {
		fmt.Fprintf(c.std.Stderr, "%s %s -> %s\n", req.Method, rel, maskedURL(req.URL, mask))
	}
}

// resolveSignIn reads the values of --sign-in, in the same syntax as a step, before anything is sent.
// They are not templates: there is nothing yet for a {name} to be filled from.
func (c *cmd) resolveSignIn(r *resolver) ([]value, error) {
	if c.f.signIn == "" {
		return nil, nil
	}
	words, err := splitWords(c.f.signIn)
	if err != nil {
		return nil, usagef("--sign-in: %v", err)
	}
	if len(words) == 0 {
		return nil, usagef("--sign-in needs the values for the sign-in rel, such as --sign-in 'credential=env:CRED'.")
	}

	vals := make([]value, 0, len(words))
	for _, w := range words {
		a, err := parseArg(w)
		if err != nil {
			return nil, usagef("--sign-in: %v", err)
		}
		if a.Kind == kindText && len(templateNames(a.Raw)) > 0 {
			return nil, usagef("--sign-in: %s cannot use {name}, because nothing has been kept yet when a sign-in happens.", a.Name)
		}
		v, err := r.resolve(a)
		if err != nil {
			return nil, err
		}
		vals = append(vals, v)
	}

	return vals, nil
}

// signInHook is what surf.WithSignIn runs when a step is refused with 401: follow the sign-in rel,
// from the 401 page if it offers one and otherwise from the front door, and return the token from
// the response.
func (c *cmd) signInHook(vals []value, root **surf.Page) surf.SignInFunc {
	return func(ctx context.Context, from *surf.Page) (string, error) {
		var front *surf.Page
		if root != nil {
			front = *root
		}
		page, err := pickPage(from, front, "sign-in")
		if err != nil {
			return "", err
		}
		values, mask, err := buildValues(vals, nil)
		if err != nil {
			return "", err
		}
		req, err := page.Prepare(ctx, "sign-in", surf.With(values))
		if err != nil {
			return "", withHint(err)
		}
		c.announce(req, "sign-in", mask)

		resp, err := page.Submit(ctx, "sign-in", values)
		if err != nil {
			return "", err
		}
		token, ok := lookup(resp.RawBody(), c.f.tokenPath)
		if !ok || token == "" {
			return "", usagef("The sign-in response has no token at %s. Say where it is with --token-path.", c.f.tokenPath)
		}

		return token, nil
	}
}

// resolveStep finds the value of each field of a step.
func (c *cmd) resolveStep(r *resolver, st step) ([]value, error) {
	out := make([]value, 0, len(st.Args)+len(st.Secrets))
	for _, a := range st.Args {
		v, err := r.resolve(a)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	for _, name := range st.Secrets {
		text, err := c.askSecret(r, name)
		if err != nil {
			return nil, err
		}
		out = append(out, value{Name: name, V: text, Secret: true})
	}

	return out, nil
}

// askSecret gets a secret that was named with --secret, so it never appears on the command line, in
// shell history or in the process list. At a terminal it is prompted for with the typing hidden. If
// typing cannot be hidden the command refuses, rather than show a secret as it is typed. Not at a
// terminal, it is one line of standard input.
func (c *cmd) askSecret(r *resolver, name string) (string, error) {
	if !c.std.StdinIsTerminal {
		return r.readStdinLine(name)
	}
	if c.std.ReadSecret == nil {
		return "", usagef("Unable to ask for %s: typing cannot be hidden here. Pipe the secret in on standard input instead.", name)
	}
	text, err := c.std.ReadSecret(name + ": ")
	if err != nil {
		return "", usagef("Unable to read %s: %v.", name, err)
	}
	if text == "" {
		return "", usagef("Nothing was entered for %s.", name)
	}

	return text, nil
}

// pickPage finds the page that offers a rel: the last response first, then the front door. A step
// names what to do, and what to do is often offered at the front door and not on the last response.
func pickPage(cur, root *surf.Page, rel string) (*surf.Page, error) {
	if cur != nil && cur.Has(rel) {
		return cur, nil
	}
	if root != nil && root.Has(rel) {
		return root, nil
	}

	var last, front []string
	if cur != nil {
		last = cur.Rels()
	}
	if root != nil {
		front = root.Rels()
	}

	return nil, usagef("Unable to follow %q. The last response offers: %s. The front door offers: %s.",
		rel, listOrNothing(last), listOrNothing(front))
}

func listOrNothing(rels []string) string {
	if len(rels) == 0 {
		return "nothing"
	}

	return strings.Join(rels, ", ")
}

// buildValues turns a step's resolved fields into what the surfer sends, filling any {name} from the
// values earlier steps kept. It also returns the mask: for each field that must not be shown, what to
// show instead. A secret field is <secret>, and a value read from a file shows the file it came from.
func buildValues(vals []value, kept map[string]keptValue) (surf.Values, map[string]string, error) {
	out := surf.Values{}
	mask := map[string]string{}
	for _, v := range vals {
		secret := v.Secret
		out[v.Name] = v.V
		if text, ok := v.V.(string); ok && v.Template {
			expanded, expandedSecret, err := expandTemplate(text, kept)
			if err != nil {
				return nil, nil, err
			}
			out[v.Name], secret = expanded, secret || expandedSecret
		}
		switch {
		case secret:
			mask[v.Name] = secretMask
		case v.File != "":
			mask[v.Name] = "<@" + v.File + ">"
		}
	}

	return out, mask, nil
}

// dryRun shows the first step's request and sends nothing. The steps after it depend on what the
// step before returns, which a dry run does not have, so they are only named.
func (c *cmd) dryRun(ctx context.Context, root *surf.Page, steps []step, resolved [][]value) int {
	for i, st := range steps {
		if i > 0 {
			fmt.Fprintf(c.std.Stdout, "then %s: not planned (it depends on what the step before returns)\n", st.Rel)
			continue
		}
		page, err := pickPage(root, root, st.Rel)
		if err != nil {
			return c.usage(err)
		}
		vals, mask, err := buildValues(resolved[i], map[string]keptValue{})
		if err != nil {
			return c.usage(err)
		}
		req, err := page.Prepare(ctx, st.Rel, surf.With(vals))
		if err != nil {
			return c.usage(withHint(err))
		}
		printPlan(c.std.Stdout, st.Rel, req, mask)
	}

	return ExitOK
}

// fail reports a step that did not succeed and returns its exit code. A server's refusal goes to
// stderr in the same layout as a success, and with --json the envelope still goes to stdout, so a
// script can read the problems.
func (c *cmd) fail(page *surf.Page, err error) int {
	code := exitFor(err)

	// A failed sign-in is the cause, so say that, rather than show the 401 page it was meant to cure.
	if strings.Contains(err.Error(), "Signing in again") {
		fmt.Fprintln(c.std.Stderr, err)
		return code
	}

	if code == ExitRefused || code == ExitServer {
		if c.f.json && page != nil {
			fmt.Fprintln(c.std.Stdout, strings.TrimSpace(string(page.RawBody())))
		}
		switch {
		case c.f.json:
			fmt.Fprintln(c.std.Stderr, err)
		case page != nil && page.Resource != nil && (page.Resource.Message != "" || len(page.Resource.Problems) > 0):
			printPage(c.std.Stderr, page)
			if after := page.Header.Get("Retry-After"); after != "" {
				fmt.Fprintf(c.std.Stderr, "  retry after: %s\n", after)
			}
		default:
			fmt.Fprintln(c.std.Stderr, err)
		}
		return code
	}

	fmt.Fprintln(c.std.Stderr, withHint(err))
	return code
}

// finish prints the last response in the chosen form.
func (c *cmd) finish(page *surf.Page) int {
	switch {
	case c.f.json:
		fmt.Fprintln(c.std.Stdout, strings.TrimSpace(string(page.RawBody())))
	case c.f.query != "":
		text, ok := lookup(page.RawBody(), c.f.query)
		if !ok {
			return c.usage(usagef("Unable to read %s from the response: there is no value there.", c.f.query))
		}
		fmt.Fprintln(c.std.Stdout, text)
	default:
		printPage(c.std.Stdout, page)
	}

	return ExitOK
}
