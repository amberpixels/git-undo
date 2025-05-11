package command

import (
	"fmt"
	"strings"
)

// AddUndoer handles undoing git add operations
type AddUndoer struct {
	args []string
}

// Undo unstages files that were added
func (a *AddUndoer) Undo(verbose bool) bool {
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
		if verbose {
			fmt.Println("Undoing git add with 'git restore --staged .'")
		}
		return ExecCommand("restore", "--staged", ".") == nil
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
		if verbose {
			fmt.Println("Undoing git add with 'git restore --staged .'")
		}
		return ExecCommand("restore", "--staged", ".") == nil
	}

	if verbose {
		fmt.Printf("Undoing git add with 'git restore --staged %s'\n", strings.Join(filesToRestore, " "))
	}

	args := append([]string{"restore", "--staged"}, filesToRestore...)
	return ExecCommand(args...) == nil
}
