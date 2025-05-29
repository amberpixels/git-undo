package githelpers

import (
	"errors"
	"fmt"
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
	Name          string      // e.g. "branch"
	Args          []string    // flags and operands
	Valid         bool        // was Name in our lookup?
	Type          CommandType // Porcelain, Plumbing, or Unknown
	IsReadOnly    bool
	ValidationErr error // any parse / lookup error
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

// IsReadOnlyGitCommand checks if a git command is read-only based on its name and arguments.
func IsReadOnlyGitCommand(command string) bool {
	parsed := ParseGitCommand(command)
	return parsed.Valid && parsed.IsReadOnly
}

// String returns a human-readable representation of the command.
func (c *GitCommand) String() string {
	return fmt.Sprintf("%s %s", c.Name, strings.Join(c.Args, " "))
}

// Normalize normalizes the command to a canonical form.
func (c *GitCommand) Normalize() *GitCommand {
	// TODO:CURSOR implement from logger.go
	return nil
}
