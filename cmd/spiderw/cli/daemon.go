package cli

import (
	"context"
	"fmt"
)

// Result wrappers for JSON-stable CLI output.
type daemonVersionResult struct {
	Version string `json:"Version"`
}

type daemonStateDirResult struct {
	StateDirectory string `json:"StateDirectory"`
}

type daemonNetConfResult struct {
	Enabled bool `json:"NetworkConfigurationEnabled"`
}

// String returns the CLI string form of the value.
func (r daemonVersionResult) String() string {
	return r.Version
}

// String returns the CLI string form of the value.
func (r daemonStateDirResult) String() string {
	return r.StateDirectory
}

// String returns the CLI string form of the value.
func (r daemonNetConfResult) String() string {
	return fmt.Sprintf("%t", r.Enabled)
}

func daemonCommand(app *App) *Command {
	return &Command{
		Name:        "daemon",
		Description: "Inspect and query the iwd daemon",
		Subcommands: map[string]*Command{
			"info": {
				Name:        "daemon info",
				Description: "Show full daemon information",
				Execute: func(args []string) error {
					if len(args) > 0 {
						return fmt.Errorf("unknown daemon info argument: %s", args[0])
					}
					ctx := context.Background()
					return app.withClient(ctx, func(client clientAPI) error {
						info, err := client.Daemon().Info(ctx)
						if err != nil {
							return err
						}

						return app.printOutput(info)
					})
				},
			},
			"version": {
				Name:        "daemon version",
				Description: "Print daemon version",
				Execute: func(args []string) error {
					if len(args) > 0 {
						return fmt.Errorf("unknown daemon version argument: %s", args[0])
					}
					ctx := context.Background()
					return app.withClient(ctx, func(client clientAPI) error {
						version, err := client.Daemon().Version(ctx)
						if err != nil {
							return err
						}

						return app.printOutput(daemonVersionResult{Version: version})
					})
				},
			},
			"state-dir": {
				Name:        "daemon state-dir",
				Description: "Print daemon state directory",
				Execute: func(args []string) error {
					if len(args) > 0 {
						return fmt.Errorf("unknown daemon state-dir argument: %s", args[0])
					}
					ctx := context.Background()
					return app.withClient(ctx, func(client clientAPI) error {
						dir, err := client.Daemon().StateDirectory(ctx)
						if err != nil {
							return err
						}

						return app.printOutput(daemonStateDirResult{StateDirectory: dir})
					})
				},
			},
			"net-conf": {
				Name:        "daemon net-conf",
				Description: "Check network configuration support",
				Execute: func(args []string) error {
					if len(args) > 0 {
						return fmt.Errorf("unknown daemon net-conf argument: %s", args[0])
					}
					ctx := context.Background()
					return app.withClient(ctx, func(client clientAPI) error {
						enabled, err := client.Daemon().NetworkConfigurationEnabled(ctx)
						if err != nil {
							return err
						}

						return app.printOutput(daemonNetConfResult{Enabled: enabled})
					})
				},
			},
		},
	}
}
