package cli

import (
	"fmt"
	"runtime/debug"
)

// version is the CLI's own version, distinct from `daemon version` (which is
// iwd's). It is stamped at release time with -ldflags (see .goreleaser.yaml).
// For `go install ...@vX` and plain local builds it stays empty and is resolved
// from the build info instead.
var version = ""

// resolveVersion returns the CLI version: the ldflags-stamped value for a
// released binary, else the module version from the build info (which
// `go install` records as the tag, or "(devel)" for a plain build), else
// "unknown".
func resolveVersion() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return bi.Main.Version
	}
	return "unknown"
}

func versionCommand(app *App) *Command {
	return &Command{
		Name:        "version",
		Description: "Print the spiderw CLI version",
		Execute: func(args []string) error {
			if len(args) != 0 {
				return fmt.Errorf("usage: spiderw version")
			}
			_, err := fmt.Fprintln(app.stdout(), resolveVersion())
			return err
		},
	}
}
