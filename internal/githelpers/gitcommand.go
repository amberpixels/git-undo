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

// GitCommand represents a parsed "git …" invocation.
type GitCommand struct {
	Name       string      // e.g. "branch"
	Args       []string    // flags and operands
	Supported  bool        // was Name in our lookup?
	Type       CommandType // Porcelain, Plumbing, or Unknown
	IsReadOnly bool
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
	if name == "undo" {
		if slices.Contains(args, "--hook") {
			return &GitCommand{
				Name:       name,
				Args:       args,
				Supported:  false,
				Type:       Custom,
				IsReadOnly: false,
			}, nil
		}
	}

	typ, ok := lookup[name]
	return &GitCommand{
		Name:       name,
		Args:       args,
		Supported:  ok,
		Type:       func() CommandType { return typ }(),
		IsReadOnly: isReadOnlyCommand(name, args),
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
		Name:       c.Name,
		Args:       normalizedArgs,
		Supported:  c.Supported,
		Type:       c.Type,
		IsReadOnly: c.IsReadOnly,
	}, nil
}

type argsNormalizer func([]string) ([]string, error)

var (
	// normalizeCommitArgs normalizes commit command arguments to canonical form.
	normalizeCommitArgs = func(args []string) ([]string, error) {
		message := ""
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
				// Extract message, removing quotes
				message = strings.Trim(args[i+1], `"'`)

				//nolint:ineffassign // We're fine with this
				i++ // Skip the message argument
			case arg == "--amend":
				amend = true
			case strings.HasPrefix(arg, "-m"):
				// Handle -m"message" format
				if len(arg) > 2 {
					message = strings.Trim(arg[2:], `"'`)
				}
				// Ignore other flags like --verbose, --signoff, etc.
			}
		}

		// Build normalized arguments
		var result []string
		if amend {
			result = append(result, "--amend")
		} else if message != "" {
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
