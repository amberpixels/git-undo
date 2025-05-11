package command

import (
	"fmt"
)

// BranchUndoer handles undoing git branch operations
type BranchUndoer struct {
	branchName string
}

// Undo deletes a branch that was just created
func (b *BranchUndoer) Undo(verbose bool) bool {
	if verbose {
		fmt.Printf("Undoing branch creation with 'git branch -D %s'\n", b.branchName)
	}

	return ExecCommand("branch", "-D", b.branchName) == nil
}
