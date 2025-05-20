package githelpers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mattn/go-shellwords"
)

func IsReadOnlyGitCommand(command string) bool {
	parsed := ParseGitCommand(command)
	return parsed.Valid && parsed.IsReadOnly
}

type CommandType int

const (
	UnknownCommand CommandType = iota
	Porcelain
	Plumbing
	Custom
)

func (ct CommandType) String() string {
	switch ct {
	case Porcelain:
		return "Porcelain"
	case Plumbing:
		return "Plumbing"
	case Custom:
		return "Custom"
	case UnknownCommand:
		fallthrough
	default:
		return "Unknown"
	}
}

// GitCommand represents a parsed "git …" invocation.
type GitCommand struct {
	Name          string      // e.g. "branch"
	Args          []string    // flags and operands
	Valid         bool        // was Name in our lookup?
	Type          CommandType // Porcelain, Plumbing, or Unknown
	IsReadOnly    bool
	ValidationErr error // any parse / lookup error
}

// Suggested name for the new field: IsReadOnly
//
// The logic below treats some verbs as always-mutating,
// and others as "conditional" (e.g. branch, checkout) that only
// mutate when given a target name.

// alwaysMutating are commands that always change state.
var alwaysMutating = map[string]struct{}{
	"add":         {},
	"am":          {},
	"archive":     {}, // e.g. archive --format=zip
	"checkout":    {}, // ditto
	"commit":      {},
	"fetch":       {}, // writes to .git/FETCH_HEAD
	"init":        {},
	"merge":       {},
	"mv":          {},
	"pull":        {}, // but what if nothing to pull?
	"push":        {}, // but what if nothing to push?
	"rebase":      {},
	"reset":       {},
	"revert":      {},
	"rm":          {},
	"stash":       {},
	"submodule":   {}, // e.g. submodule add/update
	"worktree":    {}, // add/remove
	"cherry-pick": {},
	"clone":       {},
}

// conditionalMutating are commands that only mutate if they
// have a non-flag argument (e.g. "git branch foo" vs "git branch").
var conditionalMutating = map[string]struct{}{
	"branch":   {},
	"checkout": {},
	"restore":  {},
	"switch":   {}, // newer porcelain alias for checkout
	"tag":      {},
	"remote":   {},
	"config":   {},
	"undo":     {},
}

// porcelainCommands is the list of "user-facing" verbs (main porcelain commands).
var porcelainCommands = []string{
	"add", "am", "archive", "bisect", "blame", "branch", "bundle",
	"checkout", "cherry", "cherry-pick", "citool", "clean", "clone",
	"commit", "describe", "diff", "fetch", "format-patch", "gc",
	"grep", "gui", "help", "init", "log", "merge", "mv", "notes",
	"pull", "push", "rebase", "reflog", "remote", "reset", "revert",
	"rm", "shortlog", "show", "stash", "status", "submodule", "tag",
	"worktree", "config", "restore",
	"undo",
}

// plumbingCommands is the list of low-level plumbing verbs.
var plumbingCommands = []string{
	"apply-mailbox", "apply-patch", "cat-file", "check-attr", "check-ignore",
	"check-mailmap", "check-ref-format", "checkout-index", "commit-tree",
	"diff-files", "diff-index", "diff-tree", "fast-export", "fast-import",
	"fmt-merge-msg", "for-each-ref", "hash-object", "http-backend",
	"index-pack", "init-db", "log-tree", "ls-files", "ls-remote", "ls-tree",
	"merge-base", "merge-index", "merge-tree", "mktag", "mktree",
	"pack-objects", "pack-redundant", "pack-refs", "patch-id",
	"prune", "receive-pack", "remote-ext", "replace", "rev-list",
	"rev-parse", "send-pack", "show-index", "show-ref", "symbolic-ref",
	"unpack-file", "unpack-objects", "update-index", "update-ref",
	"verify-commit", "verify-pack", "verify-tag", "write-tree",
	"name-rev",
}

// customCommands is the list of custom commands (third-party plugins).
var customCommands = []string{
	"undo",
}

// buildLookup builds a map from verb → its CommandType.
func buildLookup() map[string]CommandType {
	m := make(map[string]CommandType, len(porcelainCommands)+len(plumbingCommands))
	for _, cmd := range porcelainCommands {
		m[cmd] = Porcelain
	}
	for _, cmd := range plumbingCommands {
		m[cmd] = Plumbing
	}
	for _, cmd := range customCommands {
		m[cmd] = Custom
	}
	return m
}

var lookup = buildLookup()

// readOnlyFlags are flags that make a command read-only even if it's in conditionalMutating.
var readOnlyFlags = map[string]map[string]struct{}{
	"branch": {
		"-r":        {},
		"--remotes": {},
		"--list":    {},
		"--all":     {},
	},
	"checkout": {
		"-b": {}, // create and switch to new branch
	},
	"tag": {
		"-l":     {},
		"--list": {},
	},
	"config": {
		"--get":          {},
		"--list":         {},
		"-l":             {}, // short form of --list
		"--get-all":      {},
		"--get-regexp":   {},
		"--get-urlmatch": {},
	},
	"undo": {
		"--log": {},
	},
}

// readOnlySubcommands are subcommands that make a command read-only.
var readOnlySubcommands = map[string]map[string]struct{}{
	"remote": {
		"show":    {},
		"get-url": {},
		"list":    {},
	},
}

// readOnlyRevertedLogic is the list of commands where by default it's mutating but not read-only.
var readOnlyRevertedLogic = map[string]struct{}{
	"undo": {},
}

// isReadOnlyCommand determines if a git command is read-only based on its name and arguments.
func isReadOnlyCommand(name string, args []string) bool {
	// Always mutating commands are never read-only
	if _, always := alwaysMutating[name]; always {
		return false
	}

	// Check if it's a conditional mutating command
	if _, conditional := conditionalMutating[name]; conditional {
		// First check if there's a subcommand that makes it read-only
		if len(args) > 0 {
			if readOnlySubcmds, hasReadOnlySubcmds := readOnlySubcommands[name]; hasReadOnlySubcmds {
				if _, isReadOnly := readOnlySubcmds[args[0]]; isReadOnly {
					return true
				}
			}
		}

		// Check read-only flags
		if readOnlyFlagsForCmd, hasReadOnlyFlags := readOnlyFlags[name]; hasReadOnlyFlags {
			for _, arg := range args {
				if _, isReadOnly := readOnlyFlagsForCmd[arg]; isReadOnly {
					return true
				}
			}
		}

		// Check for non-flag arguments
		for _, a := range args {
			if !strings.HasPrefix(a, "-") {
				return false
			}
		}

		if _, ok := readOnlyRevertedLogic[name]; ok {
			return false
		}
	}

	// If we get here, it's either not a mutating command or all arguments are flags
	return true
}

// ParseGitCommand parses a git command string into a GitCommand struct.
func ParseGitCommand(raw string) *GitCommand {
	w := shellwords.NewParser()
	parts, err := w.Parse(raw)
	if err != nil {
		return &GitCommand{ValidationErr: fmt.Errorf("split error: %w", err)}
	}
	if len(parts) < 2 || parts[0] != "git" {
		return &GitCommand{ValidationErr: errors.New("not a git command")}
	}

	name := parts[1]
	args := parts[2:]

	// Special handling for git undo --hook
	if name == "undo" {
		for _, arg := range args {
			if arg == "--hook" {
				return &GitCommand{
					Name:          name,
					Args:          args,
					Valid:         false,
					Type:          Custom,
					IsReadOnly:    false,
					ValidationErr: errors.New("hook command not allowed"),
				}
			}
		}
	}

	typ, ok := lookup[name]

	return &GitCommand{
		Name:  name,
		Args:  args,
		Valid: ok,
		Type: func() CommandType {
			if ok {
				return typ
			}
			return UnknownCommand
		}(),
		IsReadOnly:    isReadOnlyCommand(name, args),
		ValidationErr: nil,
	}
}
