package undoer

import (
	"fmt"
	"os/exec"
	"strings"
)

// MergeUndoer handles undoing git merge operations.
type MergeUndoer struct {
	originalCmd *CommandDetails
}

var _ Undoer = &MergeUndoer{}

// GetUndoCommand returns the command that would undo the merge operation.
func (m *MergeUndoer) GetUndoCommand() (*UndoCommand, error) {
	// Check if this was a merge with conflicts
	output, err := CheckGitOutput("status")
	if err == nil && strings.Contains(output, "You have unmerged paths") {
		return nil, fmt.Errorf("%w: cannot undo merge with conflicts", ErrUndoNotSupported)
	}

	// Check if ORIG_HEAD exists (it should for a merge)
	_, err = CheckGitOutput("rev-parse", "--verify", "ORIG_HEAD")
	if err != nil {
		return nil, fmt.Errorf("ORIG_HEAD not found, cannot safely undo merge")
	}

	// Check if this was a fast-forward merge
	// We can detect this by checking if HEAD has multiple parents
	isMergeCmd := exec.Command(gitStr, "rev-parse", "-q", "--verify", "HEAD^2")
	isMerge := isMergeCmd.Run() == nil

	if !isMerge {
		// For fast-forward merges, we can just reset to ORIG_HEAD
		return &UndoCommand{
			Command:     "git reset --hard ORIG_HEAD",
			Description: "Undo fast-forward merge by resetting to ORIG_HEAD",
		}, nil
	}

	// For true merges (with a merge commit), we use --merge flag
	return &UndoCommand{
		Command:     "git reset --merge ORIG_HEAD",
		Description: "Undo merge commit by resetting to ORIG_HEAD",
		Warnings: []string{
			"This will undo the merge and restore the state before merging",
		},
	}, nil
}
