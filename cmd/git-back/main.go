package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/amberpixels/git-undo/internal/app"
	"github.com/urfave/cli/v3"
)

// version is set by the build ldflags
// The default value is "dev+dirty" but it should never be used. In success path, it's always overwritten.
var version = "dev+dirty"
var versionSource = "hardcoded"

const (
	appNameGitBack = "git-back"
)

func main() {
	// Create a context that can be cancelled with Ctrl+C
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	version, versionSource = app.HandleAppVersion(version, versionSource)

	cmd := &cli.Command{
		Name:  appNameGitBack,
		Usage: "Navigate back through git checkout/switch operations",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "help",
				Aliases: []string{"h"},
				Usage:   "Show help",
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose output",
			},
			&cli.BoolFlag{
				Name:  "dry-run",
				Usage: "Show what would be executed without running commands",
			},
			&cli.BoolFlag{
				Name:  "version",
				Usage: "Print the version",
			},
			&cli.StringFlag{
				Name:  "hook",
				Usage: "Hook command for shell integration (internal use)",
			},
			&cli.BoolFlag{
				Name:  "log",
				Usage: "Display the git-undo command log",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			a := app.NewAppGitBack(version, versionSource)

			if c.Bool("version") {
				return a.HandleVersion(c.Bool("verbose"))
			}

			return a.Run(ctx, app.RunOptions{
				Verbose:     c.Bool("verbose"),
				DryRun:      c.Bool("dry-run"),
				HookCommand: c.String("hook"),
				ShowLog:     c.Bool("log"),
				Args:        c.Args().Slice(),
			})
		},
	}

	if err := cmd.Run(ctx, os.Args); err != nil {
		app.HandleError(appNameGitBack, err)
	}
}
