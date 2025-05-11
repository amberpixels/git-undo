package command

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CommandUndoer represents an interface for undoing git commands
type CommandUndoer interface {
	// Undo performs the undo operation
	Undo(verbose bool) bool
}

// CommandDetails represents parsed git command details
type CommandDetails struct {
	Command    string
	SubCommand string
	Args       []string
}

// ParseCommand parses a git command string into a CommandDetails struct
func ParseCommand(cmdStr string) (*CommandDetails, error) {
	cmdParts := strings.Fields(cmdStr)
	if len(cmdParts) < 2 || cmdParts[0] != "git" {
		return nil, fmt.Errorf("invalid git command format: %s", cmdStr)
	}

	return &CommandDetails{
		Command:    cmdStr,
		SubCommand: cmdParts[1],
		Args:       cmdParts[2:],
	}, nil
}

// GetUndoer returns the appropriate CommandUndoer implementation for a git command
func GetUndoer(details *CommandDetails) (CommandUndoer, error) {
	switch details.SubCommand {
	case "commit":
		return &CommitUndoer{}, nil
	case "add":
		return &AddUndoer{args: details.Args}, nil
	case "branch":
		if len(details.Args) >= 1 {
			return &BranchUndoer{branchName: details.Args[0]}, nil
		}
		return nil, fmt.Errorf("branch command requires a branch name")
	default:
		return nil, fmt.Errorf("unsupported git command: %s", details.SubCommand)
	}
}

// ExecCommand executes a git command and returns its error status
func ExecCommand(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CheckGitOutput executes a git command and returns its output as string
func CheckGitOutput(args ...string) (string, error) {
	output, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
