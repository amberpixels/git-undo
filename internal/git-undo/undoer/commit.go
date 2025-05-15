package undoer

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// CommitUndoer handles undoing git commit operations.
type CommitUndoer struct {
	originalCmd *CommandDetails
}

// GetUndoCommand returns the command that would undo the commit.
func (c *CommitUndoer) GetUndoCommand() (*UndoCommand, error) {
	// Check if we're at the initial commit (no parent)
	isInitialCmd := exec.Command(gitStr, "rev-parse", "HEAD^{commit}")
	if err := isInitialCmd.Run(); err != nil {
		return nil, errors.New("this appears to be the initial commit and cannot be undone this way")
	}

	// Check if this is a merge commit
	isMergeCmd := exec.Command(gitStr, "rev-parse", "-q", "--verify", "HEAD^2")
	isMerge := isMergeCmd.Run() == nil

	if isMerge {
		return &UndoCommand{
			Command:     "git reset --merge ORIG_HEAD",
			Description: "Undo merge commit by resetting to ORIG_HEAD",
		}, nil
	}

	// Get the commit message to check if it was an amended commit
	commitMsg, err := CheckGitOutput("log", "-1", "--pretty=%B")
	if err == nil && strings.Contains(commitMsg, "[amend]") {
		return &UndoCommand{
			Command:     "git reset --soft HEAD@{1}",
			Description: "Undo amended commit by resetting to previous HEAD",
		}, nil
	}

	// Check if the commit is tagged
	tagOutput, err := CheckGitOutput("tag", "--points-at", "HEAD")
	if err == nil && tagOutput != "" {
		return &UndoCommand{
			Command:     "git reset --soft HEAD~1",
			Description: "Undo commit while keeping changes staged",
			Warnings: []string{
				fmt.Sprintf(
					"Warning: The commit being undone has the following tags: %s\nThese tags will now point to the parent commit.",
					tagOutput,
				),
			},
		}, nil
	}

	return &UndoCommand{
		Command:     "git reset --soft HEAD~1",
		Description: "Undo commit while keeping changes staged",
	}, nil
}
