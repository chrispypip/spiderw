package cli

import (
	"errors"
	"fmt"
	"io"
)

// Run executes the spiderw CLI with the given arguments and writers, returning
// a process exit code (0 on success, 1 on failure).
//
// Writers are injected rather than hardcoded to os.Stdout/os.Stderr so the CLI
// can be driven in-process by tests without spawning a subprocess.
func Run(args []string, stdout, stderr io.Writer) int {
	app, rest := parseGlobalFlags(args)
	app.Stdout = stdout
	app.Stderr = stderr
	return runApp(app, rest)
}

// runApp dispatches already-parsed args against app and maps the result to a
// process exit code. It is the shared core of Run and the in-process unit-test
// harness (which builds an App with a faked client).
func runApp(app *App, args []string) int {
	if err := rootCommand(app).Run(app, args); err != nil {
		if errors.Is(err, ErrHelpShown) {
			return 0
		}
		fmt.Fprintln(app.stderr(), err)
		return 1
	}
	return 0
}
