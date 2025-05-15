package undoer

import (
	"fmt"
	"strings"
)

// StashUndoer handles undoing git stash operations.
type StashUndoer struct {
	originalCmd *CommandDetails
}

var _ Undoer = &StashUndoer{}

// GetUndoCommand returns the command that would undo the stash operation.
func (s *StashUndoer) GetUndoCommand() (*UndoCommand, error) {
	// Check if this was a stash pop/apply operation
	for _, arg := range s.originalCmd.Args {
		if arg == "pop" || arg == "apply" {
			return nil, fmt.Errorf("%w for stash pop/apply", ErrUndoNotSupported)
		}
	}

	// For stash push or plain stash, we need to pop the stash and drop it
	// First check if we have any stashes
	output, err := CheckGitOutput("stash", "list")
	if err != nil || strings.TrimSpace(output) == "" {
		return nil, fmt.Errorf("no stashes found to undo")
	}

	// Pop the most recent stash and drop it
	return &UndoCommand{
		Command:     "git stash pop && git stash drop",
		Description: "Pop the most recent stash and remove it",
	}, nil
}
