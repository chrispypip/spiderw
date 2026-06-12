package cli

import "errors"

// ErrHelpShown is returned by Command.Run after printing usage when a global
// --help flag was provided.
var ErrHelpShown = errors.New("help shown")

// parseGlobalFlags scans args for truly global flags and returns the resulting
// app state plus args with those flags removed.
func parseGlobalFlags(args []string) (*App, []string) {
	app := newApp()
	out := make([]string, 0, len(args))

	for _, arg := range args {
		switch arg {
		case "--json", "-json":
			app.Output.JSON = true
		case "--session", "-session":
			app.Session = true
		case "--help", "-help", "-h":
			app.Help = true
		default:
			out = append(out, arg)
		}
	}

	return app, out
}
