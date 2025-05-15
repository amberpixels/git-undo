package undoer

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mattn/go-shellwords"
)

var ErrUndoNotSupported = errors.New("git undo not supported")

// gitStr holds git command string.
const gitStr = "git"

// UndoCommand represents a command that can undo a git operation.
type UndoCommand struct {
	// Command is the actual git command string to execute
	Command string
	// Warnings contains any warnings that should be shown to the user
	Warnings []string
	// Description is a human-readable description of what the command will do
	Description string
}

// Undoer represents an interface for undoing git commands.
type Undoer interface {
	// GetUndoCommand returns the command that would undo the operation
	GetUndoCommand() (*UndoCommand, error)
}

// CommandDetails represents parsed git command details.
type CommandDetails struct {
	FullCommand string // git commit -m "message"

	Command    string   // git
	SubCommand string   // commit
	Args       []string // []string{"-m", "message"}
}

func (d *CommandDetails) getFirstNonFlagArg() string {
	for _, arg := range d.Args {
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return ""
}

// parseGitCommand parses a git command string into a CommandDetails struct.
func parseGitCommand(gitCmdStr string) (*CommandDetails, error) {
	parts, err := shellwords.Parse(gitCmdStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input command: %w", err)
	}
	if len(parts) < 2 || parts[0] != gitStr {
		return nil, fmt.Errorf("invalid git command format: %s", gitCmdStr)
	}

	return &CommandDetails{
		FullCommand: gitCmdStr,
		Command:     parts[0],
		SubCommand:  parts[1],
		Args:        parts[2:],
	}, nil
}

// New returns the appropriate Undoer implementation for a git command.
func New(cmdStr string) (Undoer, error) {
	details, err := parseGitCommand(cmdStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input command: %w", err)
	}

	switch details.SubCommand {
	case "commit":
		return &CommitUndoer{}, nil
	case "add":
		return &AddUndoer{args: details.Args}, nil
	case "branch":
		// Check if this was a branch deletion operation
		for _, arg := range details.Args {
			if arg == "-d" || arg == "-D" || arg == "--delete" {
				return nil, fmt.Errorf("%w for branch deletion", ErrUndoNotSupported)
			}
		}

		return &BranchUndoer{branchName: details.getFirstNonFlagArg()}, nil
	case "checkout":
		// Handle checkout -b as branch creation
		for i, arg := range details.Args {
			if (arg == "-b" || arg == "--branch") && i+1 < len(details.Args) {
				return &BranchUndoer{branchName: details.getFirstNonFlagArg()}, nil
			}
		}

		return nil, fmt.Errorf("%w for checkout: only -b/--branch is supported", ErrUndoNotSupported)
	default:
		return nil, fmt.Errorf("%w for %s", ErrUndoNotSupported, details.SubCommand)
	}
}

// ExecGitCommand executes a git command and returns its error status.
func ExecGitCommand(subCmd string, args ...string) error {
	cmd := exec.Command(gitStr, append([]string{subCmd}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CheckGitOutput executes a git command and returns its output as string.
func CheckGitOutput(args ...string) (string, error) {
	output, err := exec.Command(gitStr, args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// ExecuteUndoCommand executes the undo command and returns its success status.
func ExecuteUndoCommand(cmd *UndoCommand) bool {
	gitCmd, err := parseGitCommand(cmd.Command)
	if err != nil {
		return false
	}

	return ExecGitCommand(gitCmd.SubCommand, gitCmd.Args...) == nil
}
