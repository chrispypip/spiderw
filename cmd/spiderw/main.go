package main

import (
	"errors"
	"fmt"
	"os"
)

func main() {
	app, args := parseGlobalFlags(os.Args[1:])

	if err := rootCommand(app).Run(app, args); err != nil {
		if errors.Is(err, ErrHelpShown) {
			os.Exit(0)
		}
		fmt.Fprintln(app.stderr(), err)
		os.Exit(1)
	}
}
