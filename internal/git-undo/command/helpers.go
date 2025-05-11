package command

import "strings"

// IsReadOnlyGitCommand checks if a git command is read-only and shouldn't be logged.
func IsReadOnlyGitCommand(cmd string) bool {
	// Extract the git subcommand and arguments
	fields := strings.Fields(cmd)
	if len(fields) < 2 {
		return true // Invalid command format, treat as read-only
	}

	subCmd := fields[1]

	// Always read-only commands
	readOnlyCommands := map[string]bool{
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

	// If it's in the always read-only list
	if readOnlyCommands[subCmd] {
		return true
	}

	// Special cases that require argument inspection
	switch subCmd {
	case "remote":
		// "git remote" or "git remote -v" or "git remote show" are read-only
		// "git remote add", "git remote remove", etc. are not read-only
		if len(fields) == 2 || // just "git remote"
			(len(fields) == 3 && (fields[2] == "-v" || fields[2] == "show" || fields[2] == "get-url")) {
			return true
		}
		return false

	case "branch":
		// "git branch" with no args or with -l/-a/-r (listing) is read-only
		// "git branch <n>" (create) or "git branch -d/-D" (delete) are not read-only
		if len(fields) == 2 || // just "git branch"
			(len(fields) >= 3 && (fields[2] == "-l" || fields[2] == "-a" || fields[2] == "-r" ||
				fields[2] == "--list" || fields[2] == "--all" || fields[2] == "--remotes")) {
			return true
		}
		return false

	case "tag":
		// "git tag" with no args or with -l (listing) is read-only
		// "git tag <n>" (create) or "git tag -d" (delete) are not read-only
		if len(fields) == 2 || // just "git tag"
			(len(fields) >= 3 && (fields[2] == "-l" || fields[2] == "--list")) {
			return true
		}
		return false

	case "config":
		// "git config --get", "git config --list" are read-only
		// "git config <key> <value>" or "git config --global" sets are not read-only
		if len(fields) >= 3 && (fields[2] == "--get" || fields[2] == "--list" ||
			fields[2] == "-l" || fields[2] == "--get-all" ||
			fields[2] == "--get-regexp" || fields[2] == "--get-urlmatch") {
			return true
		}
		return false
	}

	// All other commands are considered modifying actions
	return false
}
