package undoer

import (
	"fmt"
	"strings"
)

// CherryPickUndoer handles undoing git cherry-pick operations.
type CherryPickUndoer struct {
	git GitExec

	originalCmd *CommandDetails
}

var _ Undoer = &CherryPickUndoer{}

// GetUndoCommand returns the command that would undo the cherry-pick operation.
func (c *CherryPickUndoer) GetUndoCommand() (*UndoCommand, error) {
	// Check if this was a cherry-pick with --no-commit flag
	noCommit := false
	for _, arg := range c.originalCmd.Args {
		if arg == "--no-commit" || arg == "-n" {
			noCommit = true
			break
		}
	}

	if noCommit {
		// If --no-commit was used, the cherry-pick changes are staged but not committed
		// We undo by resetting the index
		return NewUndoCommand(c.git,
			"git reset --mixed HEAD",
			"Reset staged cherry-pick changes",
		), nil
	}

	// For committed cherry-picks, we need to remove the cherry-pick commit
	// First verify we have a HEAD commit
	currentHead, err := c.git.GitOutput("rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("cannot determine current HEAD: %w", err)
	}
	currentHead = strings.TrimSpace(currentHead)

	// Check if we're in the middle of a cherry-pick sequence
	cherryPickHead := ""
	cherryPickHeadOutput, err := c.git.GitOutput("rev-parse", "--verify", "CHERRY_PICK_HEAD")
	if err == nil {
		cherryPickHead = strings.TrimSpace(cherryPickHeadOutput)
	}

	if cherryPickHead != "" {
		// We're in the middle of a cherry-pick (probably due to conflicts)
		return NewUndoCommand(c.git,
			"git cherry-pick --abort",
			"Abort ongoing cherry-pick operation",
		), nil
	}

	// Validate that the current commit looks like it could be a cherry-pick
	// Check reflog to see if the last operation was cherry-pick
	reflogOutput, err := c.git.GitOutput("reflog", "-1", "--format=%s")
	if err == nil {
		reflogMsg := strings.TrimSpace(reflogOutput)
		if !strings.Contains(reflogMsg, "cherry-pick") {
			// Be more permissive - check if the commit message indicates cherry-pick
			commitMsg, err := c.git.GitOutput("log", "-1", "--format=%s", "HEAD")
			if err == nil {
				commitMsg = strings.TrimSpace(commitMsg)
				// Cherry-picked commits often have (cherry picked from commit ...) suffix
				if !strings.Contains(commitMsg, "cherry picked from commit") {
					return nil, fmt.Errorf("current HEAD does not appear to be a cherry-pick commit")
				}
			}
		}
	}

	// Get parent commit to reset to
	parentCommit, err := c.git.GitOutput("rev-parse", "HEAD~1")
	if err != nil {
		return nil, fmt.Errorf("cannot find parent commit: %w", err)
	}
	parentCommit = strings.TrimSpace(parentCommit)

	// Check if there are any uncommitted changes that would be preserved
	var warnings []string
	
	// Check for staged changes
	stagedOutput, err := c.git.GitOutput("diff", "--cached", "--name-only")
	if err == nil && strings.TrimSpace(stagedOutput) != "" {
		warnings = append(warnings, "This will preserve staged changes")
	}
	
	// Check for unstaged changes
	unstagedOutput, err := c.git.GitOutput("diff", "--name-only")
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
	description := fmt.Sprintf("Remove cherry-pick commit %s", shortHash)

	return NewUndoCommand(c.git, undoCommand, description, warnings...), nil
}