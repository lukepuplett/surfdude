package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
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
// It prompts on /dev/tty and not on standard output, so a prompt never ends up in a pipe. Echo is put
// back when the line is read, and also if the person presses Ctrl-C, so a terminal is never left
// silent.
func readSecret(prompt string) (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", errors.New("there is no terminal to ask on; pipe the secret in on standard input instead")
	}
	defer tty.Close()

	if err := stty(tty, "-echo"); err != nil {
		return "", errors.New("this terminal cannot hide typing; pipe the secret in on standard input instead")
	}
	restore := func() { _ = stty(tty, "echo") }
	defer func() { restore(); fmt.Fprintln(tty) }()

	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, os.Interrupt)
	defer signal.Stop(interrupted)
	go func() {
		if _, ok := <-interrupted; ok {
			restore()
			fmt.Fprintln(tty)
			os.Exit(130)
		}
	}()

	fmt.Fprint(tty, prompt)
	line, err := bufio.NewReader(tty).ReadString('\n')
	if err != nil && line == "" {
		return "", errors.New("nothing was entered")
	}

	return strings.TrimRight(line, "\r\n"), nil
}

// stty runs stty on the terminal.
func stty(tty *os.File, arg string) error {
	cmd := exec.Command("stty", arg)
	cmd.Stdin = tty
	return cmd.Run()
}
