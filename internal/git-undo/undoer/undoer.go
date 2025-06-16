package undoer

import (
	"errors"
	"fmt"
	"strings"

	"github.com/amberpixels/git-undo/internal/githelpers"
)

// Undoer represents an interface for undoing git commands.
type Undoer interface {
	// GetUndoCommands returns the commands that would undo the operation
	// Some operations may require multiple git commands to undo properly
	GetUndoCommands() ([]*UndoCommand, error)
}

// GitExec represents an interface for executing git commands.
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

// NewUndoCommand creates a new UndoCommand instance.
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
	case "switch":
		return &SwitchUndoer{originalCmd: cmdDetails, git: gitExec}
	case "stash":
		return &StashUndoer{originalCmd: cmdDetails, git: gitExec}
	case "merge":
		return &MergeUndoer{originalCmd: cmdDetails, git: gitExec}
	case "rm":
		return &RmUndoer{originalCmd: cmdDetails, git: gitExec}
	case "mv":
		return &MvUndoer{originalCmd: cmdDetails, git: gitExec}
	case "tag":
		return &TagUndoer{originalCmd: cmdDetails, git: gitExec}
	case "restore":
		return &RestoreUndoer{originalCmd: cmdDetails, git: gitExec}
	case "reset":
		return &ResetUndoer{originalCmd: cmdDetails, git: gitExec}
	case "revert":
		return &RevertUndoer{originalCmd: cmdDetails, git: gitExec}
	case "cherry-pick":
		return &CherryPickUndoer{originalCmd: cmdDetails, git: gitExec}
	case "clean":
		return &CleanUndoer{originalCmd: cmdDetails, git: gitExec}
	default:
		return &InvalidUndoer{rawCommand: cmdStr}
	}
}

// parseGitCommand parses a git command string into a CommandDetails struct.
func parseGitCommand(gitCmdStr string) (*CommandDetails, error) {
	parsed, err := githelpers.ParseGitCommand(gitCmdStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input git command: %w", err)
	}
	if !parsed.Supported {
		return nil, fmt.Errorf("unsupported git command format: %s", gitCmdStr)
	}

	return &CommandDetails{
		FullCommand: gitCmdStr,
		Command:     "git",
		SubCommand:  parsed.Name,
		Args:        parsed.Args,
	}, nil
}
