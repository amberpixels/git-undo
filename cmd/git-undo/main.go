package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/amberpixels/git-undo/internal/app"
)

// version is set by the build ldflags
// The default value is "dev+dirty" but it should never be used. In success path, it's always overwritten.
var version = "dev+dirty"

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

	// When running binary that was installed via `go install`, here we'll get the proper version
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		version = bi.Main.Version
	}
	application := app.New(version, verbose, dryRun)

	if err := application.Run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, redColor+appNameGitUndo+" ❌: "+grayColor+err.Error()+resetColor)
		os.Exit(1)
	}
}

const (
	grayColor  = "\033[90m"
	redColor   = "\033[31m"
	resetColor = "\033[0m"

	appNameGitUndo = "git-undo"
)
