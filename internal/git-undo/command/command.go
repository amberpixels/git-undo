package command

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

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

// Details represents parsed git command details.
type Details struct {
	Command    string
	SubCommand string
	Args       []string
}

// Parse parses a git command string into a CommandDetails struct.
func Parse(cmdStr string) (*Details, error) {
	cmdParts := strings.Fields(cmdStr)
	if len(cmdParts) < 2 || cmdParts[0] != gitStr {
		return nil, fmt.Errorf("invalid git command format: %s", cmdStr)
	}

	return &Details{
		Command:    cmdStr,
		SubCommand: cmdParts[1],
		Args:       cmdParts[2:],
	}, nil
}

// GetUndoer returns the appropriate Undoer implementation for a git command.
func GetUndoer(details *Details) (Undoer, error) {
	switch details.SubCommand {
	case "commit":
		return &CommitUndoer{}, nil
	case "add":
		return &AddUndoer{args: details.Args}, nil
	case "branch":
		if len(details.Args) >= 1 {
			return &BranchUndoer{branchName: details.Args[0]}, nil
		}
		return nil, errors.New("branch command requires a branch name")
	default:
		return nil, fmt.Errorf("unsupported git command: %s", details.SubCommand)
	}
}

// ExecCommand executes a git command and returns its error status.
func ExecCommand(args ...string) error {
	cmd := exec.Command(gitStr, args...)
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
	args := strings.Fields(cmd.Command)
	if len(args) < 2 || args[0] != gitStr {
		return false
	}
	return ExecCommand(args[1:]...) == nil
}
