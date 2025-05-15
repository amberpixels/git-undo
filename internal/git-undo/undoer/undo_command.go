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
	FullCommand string   // git commit -m "message"
	Command     string   // git
	SubCommand  string   // commit
	Args        []string // []string{"-m", "message"}
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
		return nil, fmt.Errorf("failed to parse input git command: %w", err)
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

// InvalidUndoer represents an undoer for commands that cannot be parsed or are not supported
type InvalidUndoer struct {
	rawCommand string
	parseError error
}

func (i *InvalidUndoer) GetUndoCommand() (*UndoCommand, error) {
	if i.parseError != nil {
		return nil, fmt.Errorf("%w: %w", ErrUndoNotSupported, i.parseError)
	}
	return nil, fmt.Errorf("%w: %s", ErrUndoNotSupported, i.rawCommand)
}

// New returns the appropriate Undoer implementation for a git command.
func New(cmdStr string) Undoer {
	cmdDetails, err := parseGitCommand(cmdStr)
	if err != nil {
		return &InvalidUndoer{rawCommand: cmdStr, parseError: err}
	}

	switch cmdDetails.SubCommand {
	case "commit":
		return &CommitUndoer{originalCmd: cmdDetails}
	case "add":
		return &AddUndoer{originalCmd: cmdDetails}
	case "branch":
		return &BranchUndoer{originalCmd: cmdDetails}
	case "checkout":
		return &CheckoutUndoer{originalCmd: cmdDetails}
	case "stash":
		return &StashUndoer{originalCmd: cmdDetails}
	case "merge":
		return &MergeUndoer{originalCmd: cmdDetails}
	default:
		return &InvalidUndoer{rawCommand: cmdStr}
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
