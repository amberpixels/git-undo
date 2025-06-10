package undoer

import (
	"fmt"
	"strings"
)

// RestoreUndoer handles undoing git restore operations.
type RestoreUndoer struct {
	git GitExec

	originalCmd *CommandDetails
}

var _ Undoer = &RestoreUndoer{}

// GetUndoCommand returns the command that would undo the restore operation.
func (r *RestoreUndoer) GetUndoCommand() (*UndoCommand, error) {
	// Parse flags to understand what was restored
	var isStaged bool
	var isWorktree bool
	var sourceRef string
	var files []string

	skipNext := false
	for _, arg := range r.originalCmd.Args {
		if skipNext {
			skipNext = false
			continue
		}

		switch {
		case arg == "--staged" || arg == "-S":
			isStaged = true
		case arg == "--worktree" || arg == "-W":
			isWorktree = true
		case arg == "--source" || arg == "-s":
			skipNext = true
			// Next argument will be the source ref
		case strings.HasPrefix(arg, "--source="):
			sourceRef = strings.TrimPrefix(arg, "--source=")
		case strings.HasPrefix(arg, "-s="):
			sourceRef = strings.TrimPrefix(arg, "-s=")
		case strings.HasPrefix(arg, "-"):
			// Skip other flags we don't handle
		default:
			// This is a file argument
			files = append(files, arg)
		}
	}

	// If neither --staged nor --worktree specified, git restore defaults to --worktree
	if !isStaged && !isWorktree {
		isWorktree = true
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files found in restore command: %s", r.originalCmd.FullCommand)
	}

	// The undo strategy depends on what was restored:
	// 1. If --staged was used: files were unstaged, so re-add them
	// 2. If --worktree was used: files were restored from index/HEAD, harder to undo
	// 3. If --source was used: files were restored from specific ref, very hard to undo

	if sourceRef != "" {
		// This is complex - files were restored from a specific commit
		// We would need to know their previous state, which is difficult
		return nil, fmt.Errorf(
			"%w: cannot undo git restore with --source (previous state unknown)",
			ErrUndoNotSupported,
		)
	}

	if isStaged && !isWorktree {
		// Only --staged was used: files were unstaged from index
		// Undo: re-add the files to staging area
		return NewUndoCommand(r.git,
			fmt.Sprintf("git add %s", strings.Join(files, " ")),
			fmt.Sprintf("Re-stage files: %s", strings.Join(files, ", ")),
		), nil
	}

	if isWorktree {
		// Working tree was restored (either alone or with --staged)
		// This is tricky because we don't know the previous working tree state
		// The best we can do is try to restore from git stash if available,
		// but that's not reliable. For now, we'll return an error with guidance.

		var warnings []string
		warnings = append(warnings, "Working tree changes cannot be automatically undone")
		warnings = append(warnings, "Consider using 'git stash' before 'git restore' in the future")
		warnings = append(warnings, "You may be able to recover using 'git reflog' or your editor's history")

		return NewUndoCommand(r.git,
			"echo 'Cannot automatically undo git restore --worktree'",
			"Cannot undo working tree restoration",
			warnings...,
		), fmt.Errorf("%w: cannot undo git restore --worktree (previous working tree state unknown)", ErrUndoNotSupported)
	}

	// Should not reach here, but just in case
	return nil, fmt.Errorf("%w: unhandled git restore scenario", ErrUndoNotSupported)
}
