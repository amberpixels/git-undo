package shared

import (
	"github.com/urfave/cli/v3"
)

// CommonFlags returns the standard set of CLI flags used by both git-undo and git-back commands.
func CommonFlags() []cli.Flag {
	return []cli.Flag{
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
	}
}