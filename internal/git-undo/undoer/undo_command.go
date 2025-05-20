package undoer

import (
	"errors"
	"fmt"
	"strings"

	"github.com/amberpixels/git-undo/internal/githelpers"
)

type GitExec interface {
	GitRun(subCmd string, args ...string) error
	GitOutput(subCmd string, args ...string) (string, error)
}

var ErrUndoNotSupported = errors.New("git undo not supported")

// UndoCommand represents a command that can undo a git operation.
type UndoCommand struct {
	// Command is the actual git command string to execute
	Command string
	// Warnings contains any warnings that should be shown to the user
	Warnings []string
	// Description is a human-readable description of what the command will do
	Description string

	git GitExec
}

func NewUndoCommand(git GitExec, cmdStr string, description string, warnings ...string) *UndoCommand {
	return &UndoCommand{
		Command:     cmdStr,
		Description: description,
		Warnings:    warnings,
		git:         git,
	}
}

// Exec executes the undo command and returns its success status.
func (cmd *UndoCommand) Exec() error {
	gitCmd, err := parseGitCommand(cmd.Command)
	if err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	return cmd.git.GitRun(gitCmd.SubCommand, gitCmd.Args...)
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
	parsed := githelpers.ParseGitCommand(gitCmdStr)
	if parsed.ValidationErr != nil {
		return nil, fmt.Errorf("failed to parse input git command: %w", parsed.ValidationErr)
	}
	if !parsed.Valid {
		return nil, fmt.Errorf("invalid git command format: %s", gitCmdStr)
	}

	return &CommandDetails{
		FullCommand: gitCmdStr,
		Command:     "git",
		SubCommand:  parsed.Name,
		Args:        parsed.Args,
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
func New(cmdStr string, gitExec GitExec) Undoer {
	cmdDetails, err := parseGitCommand(cmdStr)
	if err != nil {
		return &InvalidUndoer{rawCommand: cmdStr, parseError: err}
	}

	switch cmdDetails.SubCommand {
	case "commit":
		return &CommitUndoer{originalCmd: cmdDetails, git: gitExec}
	case "add":
		return &AddUndoer{originalCmd: cmdDetails, git: gitExec}
	case "branch":
		return &BranchUndoer{originalCmd: cmdDetails, git: gitExec}
	case "checkout":
		return &CheckoutUndoer{originalCmd: cmdDetails, git: gitExec}
	case "stash":
		return &StashUndoer{originalCmd: cmdDetails, git: gitExec}
	case "merge":
		return &MergeUndoer{originalCmd: cmdDetails, git: gitExec}
	default:
		return &InvalidUndoer{rawCommand: cmdStr}
	}
}
