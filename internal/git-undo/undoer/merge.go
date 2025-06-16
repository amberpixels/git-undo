package undoer

import (
	"errors"
	"strings"
)

// MergeUndoer handles undoing git merge operations.
type MergeUndoer struct {
	git GitExec

	originalCmd *CommandDetails
}

var _ Undoer = &MergeUndoer{}

// GetUndoCommands returns the commands that would undo the merge operation.
func (m *MergeUndoer) GetUndoCommands() ([]*UndoCommand, error) {
	// Check if this was a merge with conflicts
	output, err := m.git.GitOutput("status")
	if err == nil && strings.Contains(output, "You have unmerged paths") {
		return []*UndoCommand{NewUndoCommand(m.git,
			"git merge --abort",
			"Abort merge and restore state before merging",
		)}, nil
	}

	// Check if ORIG_HEAD exists (it should for a merge)
	_, err = m.git.GitOutput("rev-parse", "--verify", "ORIG_HEAD")
	if err != nil {
		return nil, errors.New("ORIG_HEAD not found, cannot safely undo merge")
	}

	// Check if this was a fast-forward merge
	// We can detect this by checking if HEAD has multiple parents
	if fastForwardMergeErr := m.git.GitRun("rev-parse", "-q", "--verify", "HEAD^2"); fastForwardMergeErr != nil {
		// For fast-forward merges, we can just reset to ORIG_HEAD
		//nolint:nilerr // it's OK here
		return []*UndoCommand{NewUndoCommand(m.git,
			"git reset --hard ORIG_HEAD",
			"Undo fast-forward merge by resetting to ORIG_HEAD",
		)}, nil
	}

	// For true merges (with a merge commit), we use --merge flag
	return []*UndoCommand{NewUndoCommand(m.git,
		"git reset --merge ORIG_HEAD",
		"Undo merge commit by resetting to ORIG_HEAD",
		"This will undo the merge and restore the state before merging",
	)}, nil
}
