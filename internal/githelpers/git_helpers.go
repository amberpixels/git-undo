package githelpers

import "strings"

// readOnlyCommands contains a map of git commands that are always read-only.
var readOnlyCommands = map[string]bool{
	"status":      true,
	"log":         true,
	"blame":       true,
	"diff":        true,
	"show":        true,
	"ls-files":    true,
	"ls-remote":   true,
	"grep":        true,
	"shortlog":    true,
	"describe":    true,
	"rev-parse":   true,
	"cat-file":    true,
	"help":        true,
	"whatchanged": true,
	"reflog":      true,
	"name-rev":    true,
}

// hasFlag checks if the command arguments contain any of the specified flags.
func hasFlag(args []string, flags ...string) bool {
	for _, arg := range args {
		for _, flag := range flags {
			if arg == flag || arg == "-"+flag || arg == "--"+flag {
				return true
			}
		}
	}
	return false
}

// IsReadOnlyGitCommand checks if a git command is read-only and shouldn't be logged.
func IsReadOnlyGitCommand(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) < 2 {
		return true // Invalid command format, treat as read-only
	}

	subCmd := fields[1]
	args := fields[2:]

	// Check if it's in the always read-only list
	if readOnlyCommands[subCmd] {
		return true
	}

	// Special cases that require argument inspection
	switch subCmd {
	case "remote":
		// "git remote" or "git remote -v" or "git remote show" are read-only
		return len(args) == 0 || hasFlag(args, "v") || hasFlag(args, "show", "get-url")

	case "branch":
		// "git branch" with no args or with listing flags is read-only
		return len(args) == 0 || hasFlag(args, "l", "a", "r", "v", "vv", "verbose", "list", "all", "remotes")

	case "tag":
		// "git tag" with no args or with listing flags is read-only
		return len(args) == 0 || hasFlag(args, "l", "list")

	case "config":
		// "git config" with get/list flags is read-only
		return hasFlag(args, "get", "list", "l", "get-all", "get-regexp", "get-urlmatch")
	}

	// All other commands are considered modifying actions
	return false
}
