package undoer

import (
	"errors"
	"fmt"
	"strings"
)

// CommitUndoer handles undoing git commit operations.
type CommitUndoer struct {
	git GitExec

	originalCmd *CommandDetails
}

// GetUndoCommand returns the command that would undo the commit.
func (c *CommitUndoer) GetUndoCommand() (*UndoCommand, error) {

	if err := c.git.GitRun("rev-parse", "HEAD^{commit}"); err != nil {
		return nil, errors.New("this appears to be the initial commit and cannot be undone this way")
	}

	// Check if this is a merge commit
	if err := c.git.GitRun("rev-parse", "-q", "--verify", "HEAD^2"); err == nil {
		return NewUndoCommand(c.git,
			"git reset --merge ORIG_HEAD",
			"Undo merge commit by resetting to ORIG_HEAD",
		), nil
	}

	// Get the commit message to check if it was an amended commit
	commitMsg, err := c.git.GitOutput("log", "-1", "--pretty=%B")
	if err == nil && strings.Contains(commitMsg, "[amend]") {
		return NewUndoCommand(c.git,
			"git reset --soft HEAD@{1}",
			"Undo amended commit by resetting to previous HEAD",
		), nil
	}

	// Check if the commit is tagged
	tagOutput, err := c.git.GitOutput("tag", "--points-at", "HEAD")
	if err == nil && tagOutput != "" {
		return NewUndoCommand(c.git,
			"git reset --soft HEAD~1",
			"Undo commit while keeping changes staged",
			fmt.Sprintf(
				"Warning: The commit being undone has the following tags: %s\nThese tags will now point to the parent commit.",
				tagOutput,
			),
		), nil
	}

	return NewUndoCommand(c.git,
		"git reset --soft HEAD~1",
		"Undo commit while keeping changes staged",
	), nil
}
