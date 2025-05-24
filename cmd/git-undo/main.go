package main

import (
	"fmt"
	"os"

	gitundo "github.com/amberpixels/git-undo"
	"github.com/amberpixels/git-undo/internal/app"
)

// Build-time version information
// This can be set during build using: go build -ldflags "-X main.version=v1.0.0".
var version = "dev"

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

	application := app.New(".", version, verbose, dryRun)
	// Set embedded scripts from root package
	app.SetEmbeddedScripts(application, gitundo.GetUpdateScript(), gitundo.GetUninstallScript())

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
