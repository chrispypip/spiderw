package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// outputConfig holds global CLI output configuration.
type outputConfig struct {
	JSON bool
}

func (a *App) stdout() io.Writer {
	if a == nil || a.Stdout == nil {
		return os.Stdout
	}
	return a.Stdout
}

func (a *App) stderr() io.Writer {
	if a == nil || a.Stderr == nil {
		return os.Stderr
	}
	return a.Stderr
}

// printOutput prints either human-readable or JSON output depending on flags.
func (a *App) printOutput(v any) error {
	if a != nil && a.Output.JSON {
		enc := json.NewEncoder(a.stdout())
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}

	// Human-readable output:
	// fmt.Println respects String() when implemented and avoids Go-literal structs.
	_, err := fmt.Fprintln(a.stdout(), v)
	return err
}
