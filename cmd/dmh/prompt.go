package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// readPassword prompts on stderr and reads a password from the terminal
// without echoing it. Without a terminal (input piped in), it reads a line.
func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		fmt.Fprintln(os.Stderr)
		if err != nil && line == "" {
			return "", fmt.Errorf("reading the password: %w", err)
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	password, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading the password: %w", err)
	}
	return string(password), nil
}
