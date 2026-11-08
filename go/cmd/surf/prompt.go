package main

import (
	"errors"
	"os"
	"strings"
	"sync"
	"syscall"
)

// terminal is what the hidden prompt needs from a terminal. It is an interface so a test can stand in
// for one, block it, panic it and send it a signal, which a real terminal cannot be made to do.
type terminal interface {
	EchoOff() error
	EchoOn() error
	Write(text string)
	ReadLine() (string, error)
}

// Exit codes after a signal, by the shell's convention: 128 plus the signal number.
const (
	exitInterrupted = 130 // SIGINT, the person pressed Ctrl-C
	exitTerminated  = 143 // SIGTERM
)

// askHidden asks for one line with the typing hidden. It turns echo off, and it puts echo back on
// every way out: when the line has been read, when the read fails, when anything panics, and when a
// signal arrives while it waits. A terminal left with echo off is a silent, broken shell, so there is
// no way out that skips it.
//
// After a signal it restores echo first, and then calls exit with 130 for Ctrl-C or 143 for a
// terminate signal. exit is a parameter so a test can see the order; the real one is os.Exit, which
// does not come back, and which is why the signal path must restore echo itself: a deferred call in
// the waiting goroutine would never run.
//
// It will not fall back to showing what is typed. If echo cannot be turned off it says so, and the
// secret can be piped in on standard input instead.
func askHidden(t terminal, prompt string, signals <-chan os.Signal, exit func(code int)) (string, error) {
	if err := t.EchoOff(); err != nil {
		return "", errors.New("this terminal cannot hide typing; pipe the secret in on standard input instead")
	}

	var once sync.Once
	restore := func() {
		once.Do(func() {
			_ = t.EchoOn()
			t.Write("\n") // the Enter that ended the line was not echoed
		})
	}
	defer restore() // a line, an error and a panic all come through here

	done := make(chan struct{})
	stopped := make(chan struct{})
	// Stop the waiting below, and wait until it has stopped, before the prompt returns. Closing done
	// alone is not enough: a signal that arrives just after the return can still be picked by the
	// goroutine's select, which chooses at random when both are ready, and then exit with 130 after
	// the secret was already read. This runs before the echo is restored, because defers run last in,
	// first out.
	defer func() {
		close(done)
		<-stopped
	}()
	go func() {
		defer close(stopped)
		select {
		case sig := <-signals:
			restore()
			exit(exitCodeFor(sig))
		case <-done:
		}
	}()

	t.Write(prompt)
	line, err := t.ReadLine()
	if err != nil && line == "" {
		return "", errors.New("nothing was entered")
	}

	return strings.TrimRight(line, "\r\n"), nil
}

// exitCodeFor is the exit code for a signal: 143 for a terminate, 130 for anything else (Ctrl-C).
func exitCodeFor(sig os.Signal) int {
	if sig == syscall.SIGTERM {
		return exitTerminated
	}

	return exitInterrupted
}
