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
	if err := application.Init(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, redColor+"git-undo ❌: "+grayColor+err.Error()+resetColor)
		os.Exit(1)
	}
	if err := application.Run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, redColor+"git-undo ❌: "+grayColor+err.Error()+resetColor)
		os.Exit(1)
	}
}

const (
	grayColor  = "\033[90m"
	redColor   = "\033[31m"
	resetColor = "\033[0m"
)
