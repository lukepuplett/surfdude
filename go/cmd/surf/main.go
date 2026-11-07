// Command surf drives a hypermedia API from its front door.
//
//	surf ROOT                               show what the front door offers
//	surf ROOT REL name=value ...            take one step: find REL on the page and send it
//	surf ROOT REL ... then REL ...          take several steps, one after another
//	surf describe ROOT REL                  what does this form take?
//	surf run FILE                           run a journey written in a file
//	surf show ROOT / surf explore ROOT      read an API, or crawl it and report problems
//
// The logic is in package cli, so tests can run it without a subprocess; docs/cli.md has the full
// description, the value syntax and the exit codes.
package main

import (
	"os"

	"github.com/lukepuplett/surfdude/go/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], cli.IO{
		Stdin:           os.Stdin,
		Stdout:          os.Stdout,
		Stderr:          os.Stderr,
		StdinIsTerminal: isTerminal(os.Stdin),
		ReadSecret:      readSecret,
	}, os.Getenv))
}
