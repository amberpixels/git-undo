package undoer

import (
	"errors"
	"fmt"
	"strings"
)

// RmUndoer handles undoing git rm operations.
type RmUndoer struct {
	git GitExec

	originalCmd *CommandDetails
}

var _ Undoer = &RmUndoer{}

// GetUndoCommands returns the commands that would undo the rm operation.
func (r *RmUndoer) GetUndoCommands() ([]*UndoCommand, error) {
	// Parse flags to understand what type of removal was done
	var isCachedOnly bool
	var isRecursive bool
	var files []string

	skipNext := false
	for _, arg := range r.originalCmd.Args {
		if skipNext {
			skipNext = false
			continue
		}

		switch arg {
		case "--cached":
			isCachedOnly = true
		case "-r", "--recursive":
			isRecursive = true
		case "-f", "--force":
			// Just note it was forced, doesn't change undo logic
		case "-n", "--dry-run":
			return nil, fmt.Errorf("%w for dry-run rm operation", ErrUndoNotSupported)
		default:
			if strings.HasPrefix(arg, "-") {
				// Handle combined flags like -rf
				if strings.Contains(arg, "r") {
					isRecursive = true
				}
				// No special handling for `f` force flag
				// or other unknown flags
			} else {
				// This is a file/directory argument
				files = append(files, arg)
			}
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files found in rm command: %s", r.originalCmd.FullCommand)
	}

	// Build the undo command based on what was removed
	if isCachedOnly {
		// git rm --cached only removes from index, files still exist in working directory
		// Undo: re-add the files to the index
		return []*UndoCommand{NewUndoCommand(r.git,
			fmt.Sprintf("git add %s", strings.Join(files, " ")),
			fmt.Sprintf("Re-add files to index: %s", strings.Join(files, ", ")),
		)}, nil
	}

	// For regular git rm (removes from both index and working directory)
	// We need to restore from the last commit
	// Check if HEAD exists
	headExists := true
	if err := r.git.GitRun("rev-parse", "--verify", "HEAD"); err != nil {
		headExists = false
	}

	if !headExists {
		return nil, errors.New("cannot undo git rm: no HEAD commit exists to restore files from")
	}

	// First restore the files from HEAD, then add them to index
	var warnings []string
	if isRecursive {
		warnings = append(warnings, "This was a recursive removal - all files and subdirectories will be restored")
	}

	// Use git restore to bring back both working tree and staged versions
	return []*UndoCommand{NewUndoCommand(r.git,
		fmt.Sprintf("git restore --source=HEAD --staged --worktree %s", strings.Join(files, " ")),
		fmt.Sprintf("Restore removed files: %s", strings.Join(files, ", ")),
		warnings...,
	)}, nil
}
