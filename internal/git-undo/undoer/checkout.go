package undoer

import (
	"fmt"
)

// CheckoutUndoer handles undoing git checkout operations.
type CheckoutUndoer struct {
	git GitExec

	originalCmd *CommandDetails
}

// GetUndoCommand returns the command that would undo the checkout operation.
func (c *CheckoutUndoer) GetUndoCommand() (*UndoCommand, error) {
	// Handle checkout -b as branch creation
	for i, arg := range c.originalCmd.Args {
		if (arg == "-b" || arg == "--branch") && i+1 < len(c.originalCmd.Args) {
			branchName := c.originalCmd.Args[i+1]
			return NewUndoCommand(c.git,
				fmt.Sprintf("git branch -D %s", branchName),
				fmt.Sprintf("Delete branch '%s' created by checkout -b", branchName),
			), nil
		}
	}

	return nil, fmt.Errorf("%w for checkout: only -b/--branch is supported", ErrUndoNotSupported)
}
