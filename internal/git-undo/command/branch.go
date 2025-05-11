package command

import (
	"fmt"
)

// BranchUndoer handles undoing git branch operations
type BranchUndoer struct {
	branchName string
}

// GetUndoCommand returns the git command that would undo the branch creation
func (b *BranchUndoer) GetUndoCommand(verbose bool) (string, error) {
	if verbose {
		fmt.Printf("Will undo branch creation with 'git branch -D %s'\n", b.branchName)
	}

	return fmt.Sprintf("git branch -D %s", b.branchName), nil
}
