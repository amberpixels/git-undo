package githelpers

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/mattn/go-shellwords"
)

// CommandType is the type of a git command.
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
		return "porcelain"
	case Plumbing:
		return "plumbing"
	case Custom:
		return "custom"
	case UnknownCommand:
		fallthrough
	default:
		return "unknown"
	}
}

// BehaviorType describes how a git command behaves in terms of state changes and logging.
type BehaviorType int

const (
	UnknownBehavior BehaviorType = iota
	// Mutating commands create or modify repository state and can be undone with git-undo.
	Mutating
	// Navigating commands move between states without creating/modifying and can be undone with git-back.
	Navigating
	// ReadOnly commands just read information and should not be logged.
	ReadOnly
)

func (bt BehaviorType) String() string {
	switch bt {
	case Mutating:
		return "mutating"
	case Navigating:
		return "navigating"
	case ReadOnly:
		return "readonly"
	case UnknownBehavior:
		fallthrough
	default:
		return "unknown"
	}
}

// GitCommand represents a parsed "git …" invocation.
type GitCommand struct {
	Name         string       // e.g. "branch"
	Args         []string     // flags and operands
	Supported    bool         // was Name in our lookup?
	Type         CommandType  // Porcelain, Plumbing, or Unknown
	BehaviorType BehaviorType // Mutating, Navigating, or ReadOnly
}

// IsReadOnly returns true if the command is read-only (for backward compatibility).
func (c *GitCommand) IsReadOnly() bool {
	return c.BehaviorType == ReadOnly
}

// IsMutating returns true if the command mutates repository state.
func (c *GitCommand) IsMutating() bool {
	return c.BehaviorType == Mutating
}

// IsNavigating returns true if the command navigates between states.
func (c *GitCommand) IsNavigating() bool {
	return c.BehaviorType == Navigating
}

// ParseGitCommand parses a git command string into a GitCommand struct.
func ParseGitCommand(raw string) (*GitCommand, error) {
	parts, err := shellwords.NewParser().Parse(raw)
	if err != nil {
		return nil, errors.New("not a shell command")
	}
	if len(parts) < 2 || parts[0] != "git" {
		return nil, errors.New("not a git command")
	}

	name := parts[1]
	args := parts[2:]

	// Special handling for git undo --hook
	if name == CustomCommandUndo {
		if slices.Contains(args, "--hook") {
			return &GitCommand{
				Name:         name,
				Args:         args,
				Supported:    false,
				Type:         Custom,
				BehaviorType: Mutating,
			}, nil
		}
	}

	typ, ok := lookup[name]
	behaviorType := determineBehaviorType(name, args)

	return &GitCommand{
		Name:         name,
		Args:         args,
		Supported:    ok,
		Type:         func() CommandType { return typ }(),
		BehaviorType: behaviorType,
	}, nil
}

// String returns a human-readable representation of the command.
func (c *GitCommand) String() string {
	return strings.TrimSpace(fmt.Sprintf("%s %s %s", "git", c.Name, strings.Join(c.Args, " ")))
}

// Normalize normalizes the command to a canonical form.
func (c *GitCommand) Normalize() (*GitCommand, error) {
	if !c.Supported {
		return nil, fmt.Errorf("cannot normalize unsupported command: %s", c)
	}

	normalizer, ok := map[string]argsNormalizer{
		"commit":      normalizeCommitArgs,
		"merge":       normalizeMergeArgs,
		"rebase":      normalizeRebaseArgs,
		"cherry-pick": normalizeCherryPickArgs,
	}[c.Name]
	if !ok {
		return nil, fmt.Errorf("normalization not implemented for git command: %s", c.Name)
	}
	normalizedArgs, err := normalizer(c.Args)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize %s command: %w", c.Name, err)
	}

	return &GitCommand{
		Name:         c.Name,
		Args:         normalizedArgs,
		Supported:    c.Supported,
		Type:         c.Type,
		BehaviorType: c.BehaviorType,
	}, nil
}

type argsNormalizer func([]string) ([]string, error)

var (
	// normalizeCommitArgs normalizes commit command arguments to canonical form.
	normalizeCommitArgs = func(args []string) ([]string, error) {
		var messageParts []string
		amend := false

		n := len(args)
		if n == 0 {
			return args, nil
		}

		// Parse arguments to extract key information
		for i := range n {
			arg := args[i]
			switch {
			case arg == "-m" && i+1 < n:
				// Collect all arguments after -m that don't start with - as the message
				// This handles both quoted and unquoted commit messages
				for j := i + 1; j < n; j++ {
					nextArg := args[j]
					if strings.HasPrefix(nextArg, "-") {
						break // Stop at next flag
					}
					// Remove quotes and add to message parts
					cleanPart := strings.Trim(nextArg, `"'`)
					messageParts = append(messageParts, cleanPart)
				}

				// Skip all the message arguments we just processed
				j := i + 1
				for j < n && !strings.HasPrefix(args[j], "-") {
					j++
				}
				i = j - 1 //nolint:ineffassign,staticcheck // Skip processed arguments in the range loop
			case arg == "--amend":
				amend = true
			case strings.HasPrefix(arg, "-m"):
				// Handle -m"message" format
				if len(arg) > 2 {
					cleanMsg := strings.Trim(arg[2:], `"'`)
					messageParts = append(messageParts, cleanMsg)
				}
				// Ignore other flags like --verbose, --signoff, etc.
			}
		}

		// Build normalized arguments
		var result []string
		if amend {
			result = append(result, "--amend")
		} else if len(messageParts) > 0 {
			// Join all message parts with spaces to create the full message
			message := strings.Join(messageParts, " ")
			result = append(result, "-m", message)
		}

		return result, nil
	}

	// normalizeMergeArgs normalizes merge command arguments to canonical form.
	normalizeMergeArgs = func(args []string) ([]string, error) {
		n := len(args)
		if n == 0 {
			return args, nil
		}

		var squash, noFf, ff bool
		var branch string

		for _, arg := range args {
			switch arg {
			case "--squash":
				squash = true
			case "--no-ff":
				noFf = true
			case "--ff":
				ff = true
			case "--ff-only":
				ff = true
			default:
				// Assume it's a branch name if it doesn't start with -
				if !strings.HasPrefix(arg, "-") && branch == "" {
					branch = arg
				}
			}
		}

		// Build normalized arguments
		var result []string
		if squash {
			result = append(result, "--squash")
		} else if noFf {
			result = append(result, "--no-ff")
		} else if ff {
			result = append(result, "--ff")
		}

		if branch != "" {
			result = append(result, branch)
		}

		return result, nil
	}

	// normalizeRebaseArgs normalizes rebase command arguments to canonical form.
	normalizeRebaseArgs = func(args []string) ([]string, error) {
		n := len(args)
		if n == 0 {
			return args, nil
		}

		var interactive bool
		var branch string

		for _, arg := range args {
			switch arg {
			case "-i", "--interactive":
				interactive = true
			default:
				if !strings.HasPrefix(arg, "-") && branch == "" {
					branch = arg
				}
			}
		}

		var result []string
		if interactive {
			result = append(result, "-i")
		}
		if branch != "" {
			result = append(result, branch)
		}

		return result, nil
	}

	// normalizeCherryPickArgs normalizes cherry-pick command arguments to canonical form.
	normalizeCherryPickArgs = func(args []string) ([]string, error) {
		n := len(args)
		if n == 0 {
			return args, nil
		}

		var commit string

		for _, arg := range args {
			if !strings.HasPrefix(arg, "-") && commit == "" {
				commit = arg
				break
			}
		}

		result := make([]string, 0, n)
		if commit != "" {
			result = append(result, commit)
		}

		return result, nil
	}
)

// NormalizedString returns the normalized command as a string.
func (c *GitCommand) NormalizedString() (string, error) {
	normalized, err := c.Normalize()
	if err != nil {
		return "", err
	}

	return normalized.String(), nil
}

// determineBehaviorType determines the behavior type of a git command based on its name and arguments.
func determineBehaviorType(name string, args []string) BehaviorType {
	// Always read-only commands
	if _, readOnly := alwaysReadOnly[name]; readOnly {
		return ReadOnly
	}

	// Always mutating commands are never read-only or navigating
	if _, always := alwaysMutating[name]; always {
		return Mutating
	}

	// Check if it's a conditional command (behavior depends on arguments)
	if _, conditional := conditionalBehavior[name]; conditional {
		return determineConditionalBehavior(name, args)
	}

	// Default to read-only for unknown commands
	return ReadOnly
}

// determineConditionalBehavior determines behavior for commands that depend on their arguments.
func determineConditionalBehavior(name string, args []string) BehaviorType {
	switch name {
	case "checkout":
		return determineCheckoutBehavior(args)
	case "switch":
		return determineSwitchBehavior(args)
	case "branch":
		return determineBranchBehavior(args)
	case "tag":
		return determineTagBehavior(args)
	case "remote":
		return determineRemoteBehavior(args)
	case "config":
		return determineConfigBehavior(args)
	case CustomCommandUndo: // "undo"
		return determineUndoBehavior(args)
	case CustomCommandBack: // "back
		return determineBackBehavior(args)
	case "restore":
		// restore is always mutating when it has file arguments
		for _, arg := range args {
			if !strings.HasPrefix(arg, "-") {
				return Mutating
			}
		}
		return ReadOnly
	default:
		// For unknown conditional commands, default to read-only
		return ReadOnly
	}
}

// determineCheckoutBehavior determines if a checkout command is mutating, navigating, or read-only.
func determineCheckoutBehavior(args []string) BehaviorType {
	// Check for branch creation flags
	for i, arg := range args {
		if (arg == "-b" || arg == "--branch") && i+1 < len(args) {
			// Creates a new branch - mutating
			return Mutating
		}
	}

	// Check for non-flag arguments (branch names, commit hashes)
	// Special case: "-" is not a flag, it means "previous branch"
	for _, arg := range args {
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			// Switching to existing branch/commit - navigating
			return Navigating
		}
	}

	// Only flags, no target - read-only
	return ReadOnly
}

// determineSwitchBehavior determines if a switch command is mutating, navigating, or read-only.
func determineSwitchBehavior(args []string) BehaviorType {
	// Check for branch creation flags
	for i, arg := range args {
		if (arg == "-c" || arg == "--create" || arg == "-C" || arg == "--force-create") && i+1 < len(args) {
			// Creates a new branch - mutating
			return Mutating
		}
	}

	// Check for non-flag arguments (branch names, commit hashes)
	// Special case: "-" is not a flag, it means "previous branch"
	for _, arg := range args {
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			// Switching to existing branch/commit - navigating
			return Navigating
		}
	}

	// Only flags, no target - read-only
	return ReadOnly
}

// determineBranchBehavior determines if a branch command is mutating, navigating, or read-only.
func determineBranchBehavior(args []string) BehaviorType {
	// Check for read-only flags first
	for _, arg := range args {
		//nolint:goconst // We want to check flags as strings
		if arg == "-r" || arg == "--remotes" || arg == "--list" || arg == "--all" {
			return ReadOnly
		}
	}

	// Check for deletion flags
	for _, arg := range args {
		if arg == "-d" || arg == "-D" || arg == "--delete" {
			return Mutating // Deleting branches is mutating
		}
	}

	// Check for non-flag arguments (branch names)
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			// Creating a new branch - mutating
			return Mutating
		}
	}

	// Only flags or no arguments - read-only (lists branches)
	return ReadOnly
}

// determineTagBehavior determines if a tag command is mutating, navigating, or read-only.
func determineTagBehavior(args []string) BehaviorType {
	// Check for read-only flags
	for _, arg := range args {
		if arg == "-l" || arg == "--list" {
			return ReadOnly
		}
	}

	// Check for deletion flags
	for _, arg := range args {
		if arg == "-d" || arg == "-D" || arg == "--delete" {
			return Mutating
		}
	}

	// Check for non-flag arguments (tag names)
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			// Creating a new tag - mutating
			return Mutating
		}
	}

	// Only flags or no arguments - read-only (lists tags)
	return ReadOnly
}

// determineRemoteBehavior determines if a remote command is mutating, navigating, or read-only.
func determineRemoteBehavior(args []string) BehaviorType {
	if len(args) == 0 {
		return ReadOnly // Lists remotes
	}

	// Check for read-only subcommands
	switch args[0] {
	case "show", "get-url":
		return ReadOnly
	case "add", "remove", "set-url", "rename", "prune", "update":
		return Mutating
	default:
		// Unknown subcommand, assume read-only
		return ReadOnly
	}
}

// determineConfigBehavior determines if a config command is mutating, navigating, or read-only.
func determineConfigBehavior(args []string) BehaviorType {
	// Check for read-only flags
	for _, arg := range args {
		if arg == "--get" || arg == "--list" || arg == "-l" || arg == "--get-all" ||
			arg == "--get-regexp" || arg == "--get-urlmatch" {
			return ReadOnly
		}
	}

	// If no read-only flags and has arguments, assume mutating (setting config)
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			return Mutating
		}
	}

	// Only flags or no arguments - read-only
	return ReadOnly
}

// determineUndoBehavior determines if an undo command is mutating, navigating, or read-only.
func determineUndoBehavior(args []string) BehaviorType {
	// git undo --log (same as git back --log) are simple read-only commands (show commands log)
	if slices.Contains(args, "--log") {
		return ReadOnly
	}
	return Mutating
}

// determineBackBehavior determines if a back command is mutating, navigating, or read-only.
// behaves exactly the same as `git undo`.
var determineBackBehavior = determineUndoBehavior
