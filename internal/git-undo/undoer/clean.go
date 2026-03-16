package undoer

import (
	"fmt"
)

// CleanUndoer handles undoing git clean operations.
// Note: git clean removes untracked files, so undo requires proactive backup.
type CleanUndoer struct {
	git GitExec

	originalCmd *CommandDetails
}

var _ Undoer = &CleanUndoer{}

// GetUndoCommands returns the commands that would undo the clean operation.
func (c *CleanUndoer) GetUndoCommands() ([]*UndoCommand, error) {
	// Check if this was a dry-run clean
	for _, arg := range c.originalCmd.Args {
		if arg == "-n" || arg == "--dry-run" {
			return nil, fmt.Errorf("%w: dry-run clean operations don't modify files", ErrUndoNotSupported)
		}
	}

	return nil, fmt.Errorf("%w: git clean permanently removes untracked files that cannot be recovered",
		ErrUndoNotSupported)
}
