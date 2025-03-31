package main

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// readPassword reads a password from the terminal; overridden in tests.
var readPassword = term.ReadPassword

func promptAskPass() (password, enable string, err error) {
	fmt.Fprint(os.Stderr, "Enter switch password: ")
	pw, err := readPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", "", fmt.Errorf("read password: %w", err)
	}

	fmt.Fprint(os.Stderr, "Enter switch enable password (optional): ")
	en, err := readPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", "", fmt.Errorf("read enable password: %w", err)
	}

	return string(pw), string(en), nil
}
