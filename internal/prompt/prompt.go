// Package prompt asks questions on the terminal, for dmh and open-document.
// It is the only user of golang.org/x/term, which reads a password without
// echoing it.
package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// stdin is shared, so that a line read ahead of a password isn't lost
var stdin = bufio.NewReader(os.Stdin)

// Password shows question on stderr and reads a password from the terminal
// without echoing it. Without a terminal (input piped in), it reads a line.
func Password(question string) (string, error) {
	fmt.Fprint(os.Stderr, question)
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		line, err := readLine()
		fmt.Fprintln(os.Stderr)
		return line, err
	}
	password, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading the password: %w", err)
	}
	return string(password), nil
}

// Line shows question on stderr and reads a line
func Line(question string) (string, error) {
	fmt.Fprint(os.Stderr, question)
	return readLine()
}

func readLine() (string, error) {
	line, err := stdin.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("reading the answer: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}
