package undoer

import (
	"fmt"
	"strings"
)

var _ Undoer = &BackUndoer{}

// BackUndoer handles undoing git checkout and git switch operations by returning to the previous branch.
type BackUndoer struct {
	git GitExec

	originalCmd *CommandDetails
}

// NewBack returns the appropriate Undoer implementation for git-back (checkout/switch undo).
func NewBack(cmdStr string, gitExec GitExec) Undoer {
	cmdDetails, err := parseGitCommand(cmdStr)
	if err != nil {
		return &InvalidUndoer{rawCommand: cmdStr, parseError: err}
	}

	switch cmdDetails.SubCommand {
	case "checkout", "switch":
		return &BackUndoer{originalCmd: cmdDetails, git: gitExec}
	default:
		return &InvalidUndoer{rawCommand: cmdStr}
	}
}

// GetUndoCommands returns the commands that would undo the checkout/switch operation.
func (b *BackUndoer) GetUndoCommands() ([]*UndoCommand, error) {
	// For git-back, we want to go back to the previous branch
	// We can use "git checkout -" which switches to the previous branch

	// First, check if we have a previous branch to go back to
	prevBranch, err := b.git.GitOutput("rev-parse", "--symbolic-full-name", "@{-1}")
	if err != nil {
		return nil, fmt.Errorf("%w: no previous branch to return to", ErrUndoNotSupported)
	}

	prevBranch = strings.TrimSpace(prevBranch)
	if prevBranch == "" {
		return nil, fmt.Errorf("%w: no previous branch to return to", ErrUndoNotSupported)
	}

	// Remove the refs/heads/ prefix if present to get just the branch name
	prevBranch = strings.TrimPrefix(prevBranch, "refs/heads/")
	_ = prevBranch // TODO: fixme.. do we need prevBranch at all?

	warnings := collectWorkingDirWarnings(b.git, "branch switching", "git-back")

	// Use "git checkout -" to go back to the previous branch/commit
	return []*UndoCommand{NewUndoCommand(b.git,
		"git checkout -",
		"Switch back to previous branch/commit",
		warnings...,
	)}, nil
}
