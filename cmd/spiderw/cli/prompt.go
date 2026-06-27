package cli

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// defaultPromptPassphrase reads a passphrase from the controlling terminal
// without echoing it. It errors when stdin is not a terminal, directing the user
// to the non-interactive flags instead.
func defaultPromptPassphrase(prompt string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("cannot prompt for passphrase: stdin is not a terminal; use --passphrase or --passphrase-stdin")
	}

	fmt.Fprint(os.Stderr, prompt)
	secret, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading passphrase: %w", err)
	}
	return string(secret), nil
}
