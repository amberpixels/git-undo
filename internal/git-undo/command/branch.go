package command

import (
	"fmt"
)

// BranchUndoer handles undoing git branch operations.
type BranchUndoer struct {
	branchName string
}

// GetUndoCommand returns the command that would undo the branch creation.
func (b *BranchUndoer) GetUndoCommand() (*UndoCommand, error) {
	return &UndoCommand{
		Command:     fmt.Sprintf("git branch -D %s", b.branchName),
		Description: fmt.Sprintf("Delete branch '%s'", b.branchName),
	}, nil
}
