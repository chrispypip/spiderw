package cli

import (
	"flag"
	"fmt"
	"sort"
)

// Command is a minimal CLI command node.
type Command struct {
	// Name is the command name used in usage text and subcommand lookup.
	Name string

	// Description is the short help text printed for the command.
	Description string

	// Flags registers command-specific flags on the provided flag set.
	Flags func(*flag.FlagSet)

	// Execute runs a leaf command with parsed positional arguments.
	Execute func(args []string) error

	// Subcommands contains child commands keyed by command name.
	Subcommands map[string]*Command

	// HelpText is optional usage text printed verbatim by --help for commands
	// that dispatch their own positional arguments instead of using Subcommands
	// (so there is no subcommand map for printUsage to enumerate). It typically
	// contains a "Commands:" block.
	HelpText string
}

// Run executes the spiderw command-line interface.
func (c *Command) Run(app *App, args []string) error {
	if app == nil {
		app = newApp()
	}

	fs := flag.NewFlagSet(c.Name, flag.ExitOnError)
	fs.Usage = func() {
		c.printUsage(app)
	}
	fs.SetOutput(app.stderr())
	if c.Flags != nil {
		c.Flags(fs)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	rest := fs.Args()

	// No args: either execute (leaf) or show usage + error (non-leaf).
	if len(rest) == 0 {
		if app.Help {
			c.printUsage(app)
			return ErrHelpShown
		}
		if c.Execute != nil {
			return c.Execute(rest)
		}
		c.printUsage(app)
		if c.Name == "" {
			return fmt.Errorf("missing command")
		}
		return fmt.Errorf("missing subcommand for %s", c.Name)
	}

	// If this node has subcommands, the first arg must be a valid subcommand.
	if len(c.Subcommands) > 0 {
		if sub, ok := c.Subcommands[rest[0]]; ok {
			// With global help, descend as far as possible to print the most
			// specific usage.
			return sub.Run(app, rest[1:])
		}
		if app.Help {
			c.printUsage(app)
			return ErrHelpShown
		}
		c.printUsage(app)
		if c.Name == "" {
			return fmt.Errorf("unknown command: %s", rest[0])
		}
		return fmt.Errorf("unknown subcommand for %s: %s", c.Name, rest[0])
	}

	// Leaf node: if global help was requested, show usage for this leaf.
	if app.Help {
		c.printUsage(app)
		return ErrHelpShown
	}

	// Leaf node: pass remaining args to Execute.
	if c.Execute != nil {
		return c.Execute(rest)
	}

	c.printUsage(app)
	return fmt.Errorf("command not executable: %s", c.Name)
}

func (c *Command) printUsage(app *App) {
	out := app.stdout()
	fmt.Fprintf(out, "%s\n\n", c.Description)
	fmt.Fprintf(out, "Usage:\n  spiderw %s", c.Name)
	if len(c.Subcommands) > 0 || c.HelpText != "" {
		fmt.Fprintf(out, " <command>")
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out)

	if len(c.Subcommands) > 0 {
		fmt.Fprintln(out, "Commands:")
		// Display the invocation name (the map key, e.g. "version"), sorted for
		// stable output. sub.Name holds the fully-qualified path ("daemon
		// version") used for the subcommand's own usage line, which is too long
		// for this list.
		names := make([]string, 0, len(c.Subcommands))
		for name := range c.Subcommands {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Fprintf(out, "  %-12s %s\n", name, c.Subcommands[name].Description)
		}
	}
	if c.HelpText != "" {
		fmt.Fprintln(out, c.HelpText)
	}
}

func rootCommand(app *App) *Command {
	if app == nil {
		app = newApp()
	}

	return &Command{
		Name:        "",
		Description: "spiderw - a safe Go interface to iwd",
		Subcommands: map[string]*Command{
			"daemon":        daemonCommand(app),
			"adapter":       adapterCommand(app),
			"device":        deviceCommand(app),
			"bss":           bssCommand(app),
			"network":       networkCommand(app),
			"known-network": knownNetworkCommand(app),
		},
	}
}
