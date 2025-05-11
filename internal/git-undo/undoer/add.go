package undoer

import (
	"fmt"
	"strings"
)

// AddUndoer handles undoing git add operations.
type AddUndoer struct {
	args []string
}

var _ Undoer = &AddUndoer{}

// GetUndoCommand returns the command that would undo the add operation.
func (a *AddUndoer) GetUndoCommand() (*UndoCommand, error) {
	// Parse the arguments to handle flags properly
	// Common flags for git add: --all, -A, --update, -u, etc.

	// Check for special flags that affect what to unstage
	hasAllFlag := false
	for _, arg := range a.args {
		if arg == "--all" || arg == "-A" || arg == "--no-ignore-removal" {
			hasAllFlag = true
			break
		}
	}

	// If --all flag was used or no specific files, unstage everything
	if hasAllFlag || len(a.args) == 0 {
		return &UndoCommand{
			Command:     "git restore --staged .",
			Description: "Unstage all files",
		}, nil
	}

	// For other cases, filter out flags and only pass real file paths to restore
	var filesToRestore []string
	for _, arg := range a.args {
		// Skip any flags (arguments starting with - or --)
		if !strings.HasPrefix(arg, "-") {
			filesToRestore = append(filesToRestore, arg)
		}
	}

	// If we only had flags but no files, default to restoring everything
	if len(filesToRestore) == 0 {
		return &UndoCommand{
			Command:     "git restore --staged .",
			Description: "Unstage all files",
		}, nil
	}

	return &UndoCommand{
		Command:     fmt.Sprintf("git restore --staged %s", strings.Join(filesToRestore, " ")),
		Description: fmt.Sprintf("Unstage specific files: %s", strings.Join(filesToRestore, ", ")),
	}, nil
}
