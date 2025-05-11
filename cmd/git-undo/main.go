package main

import (
	"fmt"
	"os"

	"github.com/amberpixels/git-undo/internal/app"
)

func main() {
	var verbose, dryRun bool
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "--verbose" {
			verbose = true
		}
		if arg == "--dry-run" {
			dryRun = true
		}
	}

	application := app.New(verbose, dryRun)
	if err := application.Run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
