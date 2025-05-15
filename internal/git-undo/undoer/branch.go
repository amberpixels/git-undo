package undoer

import (
	"fmt"
)

// BranchUndoer handles undoing git branch operations.
type BranchUndoer struct {
	originalCmd *CommandDetails
}

// GetUndoCommand returns the command that would undo the branch creation.
func (b *BranchUndoer) GetUndoCommand() (*UndoCommand, error) {
	// Check if this was a branch deletion operation
	for _, arg := range b.originalCmd.Args {
		if arg == "-d" || arg == "-D" || arg == "--delete" {
			return nil, fmt.Errorf("%w for branch deletion", ErrUndoNotSupported)
		}
	}

	branchName := b.originalCmd.getFirstNonFlagArg()
	if branchName == "" {
		return nil, fmt.Errorf("no branch name found in command: %s", b.originalCmd.FullCommand)
	}

	return &UndoCommand{
		Command:     fmt.Sprintf("git branch -D %s", branchName),
		Description: fmt.Sprintf("Delete branch '%s'", branchName),
	}, nil
}
