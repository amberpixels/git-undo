package undoer

import (
	"errors"
	"fmt"
	"strings"
)

// ResetUndoer handles undoing git reset operations.
type ResetUndoer struct {
	git GitExec

	originalCmd *CommandDetails
}

var _ Undoer = &ResetUndoer{}

// GetUndoCommand returns the command that would undo the reset operation.
//
//nolint:goconst // we're having lot of string git commands here
func (r *ResetUndoer) GetUndoCommand() (*UndoCommand, error) {
	// First, get the current HEAD to know where we are now
	//TODO: do we actually need HEAD here?
	_, err := r.git.GitOutput("rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("cannot determine current HEAD: %w", err)
	}

	// Get the reflog to find the previous HEAD position
	// The reflog entry should show what HEAD was before this reset
	reflogOutput, err := r.git.GitOutput("reflog", "-n", "2", "--format=%H %s")
	if err != nil {
		return nil, fmt.Errorf("cannot access reflog to find previous state: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(reflogOutput), "\n")
	if len(lines) < 2 {
		return nil, errors.New("insufficient reflog history to undo reset")
	}

	// Parse the second line (the state before current reset)
	previousLine := strings.TrimSpace(lines[1])
	parts := strings.SplitN(previousLine, " ", 2)
	if len(parts) < 1 {
		return nil, fmt.Errorf("cannot parse reflog entry: %s", previousLine)
	}
	previousHead := parts[0]

	// Determine the reset mode from the original command
	resetMode := r.getResetMode()

	// Check if we have staged changes that would be lost
	var warnings []string
	if resetMode == "hard" {
		// Check for staged changes
		stagedOutput, err := r.git.GitOutput("diff", "--cached", "--name-only")
		if err == nil && strings.TrimSpace(stagedOutput) != "" {
			warnings = append(warnings, "This will discard all staged changes")
		}

		// Check for unstaged changes
		unstagedOutput, err := r.git.GitOutput("diff", "--name-only")
		if err == nil && strings.TrimSpace(unstagedOutput) != "" {
			warnings = append(warnings, "This will discard all unstaged changes")
		}
	}

	// Generate the appropriate undo command based on original reset mode
	var undoCommand string
	var description string

	// Helper function to safely truncate commit hash
	shortHash := getShortHash(previousHead)

	switch resetMode {
	case "soft":
		// For soft reset, we just move HEAD back
		undoCommand = fmt.Sprintf("git reset --soft %s", previousHead)
		description = fmt.Sprintf("Reset HEAD back to %s (preserving index and working tree)", shortHash)
	case "mixed", "":
		// Default is mixed reset
		undoCommand = fmt.Sprintf("git reset %s", previousHead)
		description = fmt.Sprintf("Reset HEAD and index back to %s (preserving working tree)", shortHash)
	case "hard":
		// Hard reset - most destructive, warn user
		undoCommand = fmt.Sprintf("git reset --hard %s", previousHead)
		description = fmt.Sprintf("Reset HEAD, index, and working tree back to %s", shortHash)
		warnings = append(warnings, "This will restore the working tree to the previous state")
	default:
		return nil, fmt.Errorf("%w: unsupported reset mode: %s", ErrUndoNotSupported, resetMode)
	}

	return NewUndoCommand(r.git, undoCommand, description, warnings...), nil
}

// getResetMode determines the reset mode from the original command arguments.
func (r *ResetUndoer) getResetMode() string {
	for _, arg := range r.originalCmd.Args {
		switch arg {
		case "--soft":
			return "soft"
		case "--mixed":
			return "mixed"
		case "--hard":
			return "hard"
		}
	}
	return "" // Default is mixed
}
