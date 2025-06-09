package undoer

import (
	"fmt"
	"strings"
)

// CleanUndoer handles undoing git clean operations.
// Note: git clean removes untracked files, so undo requires proactive backup.
type CleanUndoer struct {
	git GitExec

	originalCmd *CommandDetails
}

var _ Undoer = &CleanUndoer{}

// GetUndoCommand returns the command that would undo the clean operation.
func (c *CleanUndoer) GetUndoCommand() (*UndoCommand, error) {
	// Git clean is inherently destructive and removes untracked files permanently.
	// Unlike other git operations, once files are cleaned, they cannot be recovered
	// from git's internal state since they were never tracked.
	
	// Check if this was a dry-run clean
	isDryRun := false
	for _, arg := range c.originalCmd.Args {
		if arg == "-n" || arg == "--dry-run" {
			isDryRun = true
			break
		}
	}

	if isDryRun {
		// Dry-run clean doesn't actually remove files, so no undo needed
		return nil, fmt.Errorf("%w: dry-run clean operations don't modify files", ErrUndoNotSupported)
	}

	// For actual clean operations, we cannot recover the deleted files
	// because git clean removes untracked files that were never in git's database.
	// The only way to "undo" would be if we had proactively backed up untracked files
	// before the clean operation, but that would require hooks to run before clean.

	// Check if there are any backups in a potential backup directory
	// This is a future enhancement where git-undo could backup files before clean
	backupDir := ".git/git-undo/clean-backups"
	
	// Try to find the most recent backup
	backupList, err := c.git.GitOutput("ls", "-la", backupDir)
	if err != nil || strings.TrimSpace(backupList) == "" {
		// No backups available
		return nil, fmt.Errorf("%w: git clean permanently removes untracked files that cannot be recovered. "+
			"To enable git clean undo in the future, consider implementing pre-clean backup hooks", 
			ErrUndoNotSupported)
	}

	// If we had backups (future implementation), we could restore them
	// For now, this is not supported but provides a framework for future enhancement
	return nil, fmt.Errorf("%w: git clean undo requires pre-operation backup system (not yet implemented). "+
		"Lost files cannot be recovered as they were untracked", ErrUndoNotSupported)
}

// NOTE: To properly support git clean undo, we would need:
// 1. Pre-operation hooks that backup untracked files before clean
// 2. Timestamp-based backup management
// 3. Selective restore capabilities
// 4. Integration with the logging system to match clean operations with backups
//
// This would be a significant enhancement requiring:
// - Modification of the hook system to run BEFORE destructive operations
// - Backup storage and management system
// - File restoration logic
// - User interface for selective recovery