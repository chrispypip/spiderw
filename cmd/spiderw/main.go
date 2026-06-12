// Command spiderw is a small CLI for interacting with iwd through the spiderw
// public API. It is a thin wrapper around the importable cmd/spiderw/cli
// package; see that package for the CLI implementation.
package main

import (
	"os"

	"github.com/chrispypip/spiderw/cmd/spiderw/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
