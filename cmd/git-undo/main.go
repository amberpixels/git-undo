package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/amberpixels/git-undo/cmd/shared"
	"github.com/amberpixels/git-undo/internal/app"
	"github.com/urfave/cli/v3"
)

// version is set by the build ldflags
// The default value is "dev+dirty" but it should never be used. In success path, it's always overwritten.
var version = "dev+dirty"
var versionSource = "hardcoded"

const (
	appNameGitUndo = "git-undo"
)

func main() {
	// Create a context that can be cancelled with Ctrl+C
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	version, versionSource = app.HandleAppVersion(version, versionSource)

	cmd := &cli.Command{
		Name:                      appNameGitUndo,
		Usage:                     "Universal \"Ctrl + Z\" for Git commands",
		DisableSliceFlagSeparator: true,
		HideHelp:                  true,
		Flags:                     shared.CommonFlags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			application := app.NewAppGitUndo(version, versionSource)
			if c.Bool("version") {
				return application.HandleVersion(ctx, c.Bool("verbose"))
			}

			// Use the new structured approach with parsed options
			opts := app.RunOptions{
				Verbose:     c.Bool("verbose"),
				DryRun:      c.Bool("dry-run"),
				HookCommand: c.String("hook"),
				ShowLog:     c.Bool("log"),
				Args:        c.Args().Slice(),
			}

			return application.Run(ctx, opts)
		},
	}

	if err := cmd.Run(ctx, os.Args); err != nil {
		app.HandleError(appNameGitUndo, err)
	}
}
