package undoer

import (
	"fmt"
	"strings"
)

// TagUndoer handles undoing git tag operations.
type TagUndoer struct {
	git GitExec

	originalCmd *CommandDetails
}

var _ Undoer = &TagUndoer{}

// GetUndoCommand returns the command that would undo the tag creation.
func (t *TagUndoer) GetUndoCommand() (*UndoCommand, error) {
	// Check if this was a tag deletion operation
	for _, arg := range t.originalCmd.Args {
		if arg == "-d" || arg == "-D" || arg == "--delete" {
			return nil, fmt.Errorf("%w for tag deletion", ErrUndoNotSupported)
		}
	}

	// Find the tag name - it's the first non-flag argument
	tagName := ""
	skipNext := false

	for _, arg := range t.originalCmd.Args {
		if skipNext {
			skipNext = false
			continue
		}

		// Handle flags with embedded values (e.g., -m="message")
		if strings.Contains(arg, "=") && strings.HasPrefix(arg, "-") {
			continue
		}

		// Skip flags that take values as next argument
		if arg == "-a" || arg == "--annotate" {
			// These don't take values, they're just flags
			continue
		}
		if arg == "-m" || arg == "--message" ||
			arg == "-F" || arg == "--file" || arg == "-s" || arg == "--sign" ||
			arg == "-u" || arg == "--local-user" {
			skipNext = true
			continue
		}

		// Skip other flags
		if strings.HasPrefix(arg, "-") {
			continue
		}

		// This should be the tag name
		tagName = arg
		break
	}

	if tagName == "" {
		return nil, fmt.Errorf("no tag name found in command: %s", t.originalCmd.FullCommand)
	}

	// Verify the tag exists before trying to delete it
	if err := t.git.GitRun("rev-parse", "--verify", "refs/tags/"+tagName); err != nil {
		return nil, fmt.Errorf("tag '%s' does not exist, cannot undo tag creation", tagName)
	}

	return NewUndoCommand(t.git,
		fmt.Sprintf("git tag -d %s", tagName),
		fmt.Sprintf("Delete tag '%s'", tagName),
	), nil
}
