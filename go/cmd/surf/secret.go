package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// isTerminal says whether a file is a person at a keyboard, not a pipe or a file.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// readSecret asks for one line on the terminal with the typing hidden. It turns echo off with stty,
// because the standard library has no way to, and it will not fall back to showing what is typed: if
// echo cannot be turned off it says so, and the secret can be piped in on standard input instead.
//
// It prompts on /dev/tty and not on standard output, so a prompt never ends up in a pipe. The work of
// turning echo off, and of putting it back on every way out (a line, an error, a panic, Ctrl-C and a
// terminate signal), is askHidden's, which is tested without a terminal. This part only connects it
// to the real one.
func readSecret(prompt string) (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", errors.New("there is no terminal to ask on; pipe the secret in on standard input instead")
	}
	defer tty.Close()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	return askHidden(&ttyTerminal{tty: tty, reader: bufio.NewReader(tty)}, prompt, signals, os.Exit)
}

// ttyTerminal is the real terminal, /dev/tty, driven with stty because the standard library has no
// way to turn echo off.
type ttyTerminal struct {
	tty    *os.File
	reader *bufio.Reader
}

func (t *ttyTerminal) EchoOff() error { return stty(t.tty, "-echo") }
func (t *ttyTerminal) EchoOn() error  { return stty(t.tty, "echo") }
func (t *ttyTerminal) Write(text string) {
	fmt.Fprint(t.tty, text)
}
func (t *ttyTerminal) ReadLine() (string, error) { return t.reader.ReadString('\n') }

// stty runs stty on the terminal.
func stty(tty *os.File, arg string) error {
	cmd := exec.Command("stty", arg)
	cmd.Stdin = tty
	return cmd.Run()
}
