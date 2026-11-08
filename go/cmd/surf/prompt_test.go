package main

import (
	"errors"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// Feature: the hidden prompt always puts the terminal back
//
// A secret is typed with echo off. A terminal left with echo off is a silent, broken shell, so echo
// must come back on every way out: a normal line, an error, a panic, Ctrl-C and a terminate signal.

// fakeTerminal records what the prompt does to it, in order. ReadLine blocks until released, so a
// test can deliver a signal while the prompt is waiting for a person.
type fakeTerminal struct {
	mu        sync.Mutex
	calls     []string
	offErr    error
	line      string
	readErr   error
	panicWith any
	release   chan struct{}
}

func newFakeTerminal() *fakeTerminal { return &fakeTerminal{release: make(chan struct{})} }

func (f *fakeTerminal) note(s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, s)
}

func (f *fakeTerminal) log() string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return strings.Join(f.calls, ",")
}

func (f *fakeTerminal) count(s string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c == s {
			n++
		}
	}

	return n
}

func (f *fakeTerminal) EchoOff() error { f.note("echo-off"); return f.offErr }
func (f *fakeTerminal) EchoOn() error  { f.note("echo-on"); return nil }
func (f *fakeTerminal) Write(text string) {
	if text == "\n" {
		f.note("newline")
		return
	}
	f.note("write")
}

func (f *fakeTerminal) ReadLine() (string, error) {
	f.note("read")
	if f.panicWith != nil {
		panic(f.panicWith)
	}
	if f.line != "" || f.readErr != nil {
		return f.line, f.readErr
	}
	<-f.release // wait for a person, or for the test

	return f.line, f.readErr
}

// exits records the codes passed to exit, and when.
type exits struct {
	mu    sync.Mutex
	codes []int
	term  *fakeTerminal
	order []string // what the terminal had been told when exit was called
}

func (e *exits) exit(code int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.codes = append(e.codes, code)
	e.order = append(e.order, e.term.log())
}

func (e *exits) got() []int {
	e.mu.Lock()
	defer e.mu.Unlock()

	return append([]int(nil), e.codes...)
}

// waitFor polls until cond holds or a second has passed.
func waitFor(cond func() bool) bool {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(2 * time.Millisecond)
	}

	return cond()
}

// Scenario: a line typed normally comes back, and echo is turned off for it and on again after
func TestAskHidden_ReturnsTheLineAndRestoresEcho(t *testing.T) {
	term := newFakeTerminal()
	term.line = "hunter2\r\n"
	e := &exits{term: term}

	got, err := askHidden(term, "credential: ", make(chan os.Signal), e.exit)

	if err != nil || got != "hunter2" {
		t.Fatalf("got %q, %v, want hunter2 with the line ending trimmed", got, err)
	}
	if want := "echo-off,write,read,echo-on,newline"; term.log() != want {
		t.Fatalf("terminal calls = %s, want %s", term.log(), want)
	}
	if len(e.got()) != 0 {
		t.Fatalf("exit was called with %v", e.got())
	}
}

// Scenario: a terminal that cannot hide typing is refused, with the way round it, and is not read
func TestAskHidden_RefusesWhenEchoCannotBeTurnedOff(t *testing.T) {
	term := newFakeTerminal()
	term.offErr = errors.New("no stty")

	_, err := askHidden(term, "credential: ", make(chan os.Signal), (&exits{term: term}).exit)

	if err == nil || !strings.Contains(err.Error(), "cannot hide typing") || !strings.Contains(err.Error(), "standard input") {
		t.Fatalf("err = %v, want it to say typing cannot be hidden and to point at standard input", err)
	}
	if term.count("read") != 0 || term.count("write") != 0 {
		t.Fatalf("the terminal was prompted or read although echo could not be hidden: %s", term.log())
	}
}

// Scenario: a read that fails still puts echo back
func TestAskHidden_RestoresEchoWhenTheReadFails(t *testing.T) {
	term := newFakeTerminal()
	term.readErr = errors.New("closed")

	_, err := askHidden(term, "credential: ", make(chan os.Signal), (&exits{term: term}).exit)

	if err == nil {
		t.Fatal("a failed read was reported as success")
	}
	if term.count("echo-on") != 1 {
		t.Fatalf("echo was restored %d time(s), want 1: %s", term.count("echo-on"), term.log())
	}
}

// Scenario: a panic while waiting for the line still puts echo back before the program dies
func TestAskHidden_RestoresEchoWhenItPanics(t *testing.T) {
	term := newFakeTerminal()
	term.panicWith = "boom"

	func() {
		defer func() {
			if v := recover(); v != "boom" {
				t.Fatalf("recovered %v, want the panic to carry on", v)
			}
		}()
		_, _ = askHidden(term, "credential: ", make(chan os.Signal), (&exits{term: term}).exit)
	}()

	if term.count("echo-on") != 1 {
		t.Fatalf("echo was restored %d time(s) after a panic, want 1: %s", term.count("echo-on"), term.log())
	}
}

// Scenario: Ctrl-C while the prompt waits puts echo back first, and then exits 130
func TestAskHidden_CtrlCRestoresEchoThenExits130(t *testing.T) {
	term := newFakeTerminal()
	e := &exits{term: term}
	signals := make(chan os.Signal, 1)
	finished := make(chan struct{})
	go func() { _, _ = askHidden(term, "credential: ", signals, e.exit); close(finished) }()
	if !waitFor(func() bool { return term.count("read") == 1 }) {
		t.Fatal("the prompt never started waiting for a line")
	}

	signals <- os.Interrupt

	if !waitFor(func() bool { return len(e.got()) == 1 }) {
		t.Fatal("Ctrl-C did not exit")
	}
	if got := e.got()[0]; got != 130 {
		t.Fatalf("exit code = %d, want 130 (the shell's code for an interrupt)", got)
	}
	if !strings.Contains(e.order[0], "echo-on") {
		t.Fatalf("exit was called before echo was restored: the terminal had seen %s", e.order[0])
	}
	close(term.release)
	<-finished
	if term.count("echo-on") != 1 {
		t.Fatalf("echo was restored %d time(s), want exactly 1", term.count("echo-on"))
	}
}

// Scenario: a terminate signal does the same, and exits 143
func TestAskHidden_TerminateRestoresEchoThenExits143(t *testing.T) {
	term := newFakeTerminal()
	e := &exits{term: term}
	signals := make(chan os.Signal, 1)
	finished := make(chan struct{})
	go func() { _, _ = askHidden(term, "credential: ", signals, e.exit); close(finished) }()
	if !waitFor(func() bool { return term.count("read") == 1 }) {
		t.Fatal("the prompt never started waiting for a line")
	}

	signals <- syscall.SIGTERM

	if !waitFor(func() bool { return len(e.got()) == 1 }) {
		t.Fatal("SIGTERM did not exit")
	}
	if got := e.got()[0]; got != 143 {
		t.Fatalf("exit code = %d, want 143", got)
	}
	if !strings.Contains(e.order[0], "echo-on") {
		t.Fatalf("exit was called before echo was restored: %s", e.order[0])
	}
	close(term.release)
	<-finished
}

// Scenario: once the prompt has returned it no longer answers signals
func TestAskHidden_StopsListeningWhenItReturns(t *testing.T) {
	term := newFakeTerminal()
	term.line = "x\n"
	e := &exits{term: term}
	signals := make(chan os.Signal, 1)

	_, _ = askHidden(term, "credential: ", signals, e.exit)
	signals <- os.Interrupt
	time.Sleep(50 * time.Millisecond)

	if len(e.got()) != 0 {
		t.Fatalf("a signal after the prompt returned still called exit(%v)", e.got())
	}
}
