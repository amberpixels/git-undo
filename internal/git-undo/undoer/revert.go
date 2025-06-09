package undoer

import (
	"fmt"
	"strings"
)

// RevertUndoer handles undoing git revert operations.
type RevertUndoer struct {
	git GitExec

	originalCmd *CommandDetails
}

var _ Undoer = &RevertUndoer{}

// GetUndoCommand returns the command that would undo the revert operation.
func (r *RevertUndoer) GetUndoCommand() (*UndoCommand, error) {
	// Check if this was a revert with --no-commit flag
	noCommit := false
	for _, arg := range r.originalCmd.Args {
		if arg == "--no-commit" || arg == "-n" {
			noCommit = true
			break
		}
	}

	if noCommit {
		// If --no-commit was used, the revert changes are staged but not committed
		// We undo by resetting the index
		return NewUndoCommand(r.git,
			"git reset --mixed HEAD",
			"Reset staged revert changes",
		), nil
	}

	// For committed reverts, we need to identify the revert commit and remove it
	// First verify we have a HEAD commit
	currentHead, err := r.git.GitOutput("rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("cannot determine current HEAD: %w", err)
	}
	currentHead = strings.TrimSpace(currentHead)

	// Check if the current commit is indeed a revert commit
	commitMsg, err := r.git.GitOutput("log", "-1", "--format=%s", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("cannot get commit message: %w", err)
	}
	commitMsg = strings.TrimSpace(commitMsg)

	// Revert commits typically start with "Revert" 
	if !strings.HasPrefix(commitMsg, "Revert") {
		// Try to be more flexible - check if the most recent commit looks like a revert
		// by checking the reflog for revert operations
		reflogOutput, err := r.git.GitOutput("reflog", "-1", "--format=%s")
		if err == nil {
			reflogMsg := strings.TrimSpace(reflogOutput)
			if !strings.Contains(reflogMsg, "revert") {
				return nil, fmt.Errorf("current HEAD does not appear to be a revert commit, cannot safely undo")
			}
		}
	}

	// Get parent commit to reset to
	parentCommit, err := r.git.GitOutput("rev-parse", "HEAD~1")
	if err != nil {
		return nil, fmt.Errorf("cannot find parent commit: %w", err)
	}
	parentCommit = strings.TrimSpace(parentCommit)

	// Check if there are any uncommitted changes that would be lost
	var warnings []string
	
	// Check for staged changes
	stagedOutput, err := r.git.GitOutput("diff", "--cached", "--name-only")
	if err == nil && strings.TrimSpace(stagedOutput) != "" {
		warnings = append(warnings, "This will preserve staged changes")
	}
	
	// Check for unstaged changes
	unstagedOutput, err := r.git.GitOutput("diff", "--name-only")
	if err == nil && strings.TrimSpace(unstagedOutput) != "" {
		warnings = append(warnings, "This will preserve unstaged changes")
	}

	// Use soft reset to preserve working directory and staging area
	undoCommand := fmt.Sprintf("git reset --soft %s", parentCommit)
	
	// Safely truncate commit hash
	shortHash := currentHead
	if len(currentHead) > 8 {
		shortHash = currentHead[:8]
	}
	description := fmt.Sprintf("Remove revert commit %s", shortHash)

	return NewUndoCommand(r.git, undoCommand, description, warnings...), nil
}