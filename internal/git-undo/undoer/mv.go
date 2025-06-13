package undoer

import (
	"fmt"
	"strings"
)

// MvUndoer handles undoing git mv operations.
type MvUndoer struct {
	git GitExec

	originalCmd *CommandDetails
}

var _ Undoer = &MvUndoer{}

// GetUndoCommand returns the command that would undo the mv operation.
func (m *MvUndoer) GetUndoCommand() (*UndoCommand, error) {
	// Parse arguments to find source and destination
	// git mv can be: git mv <source> <dest> or git mv <source1> <source2> ... <dest_dir>

	var nonFlagArgs []string
	for _, arg := range m.originalCmd.Args {
		// Skip flags (git mv doesn't have many, but let's be safe)
		if !strings.HasPrefix(arg, "-") {
			nonFlagArgs = append(nonFlagArgs, arg)
		}
	}

	if len(nonFlagArgs) < 2 {
		return nil, fmt.Errorf("insufficient arguments for git mv: %s", m.originalCmd.FullCommand)
	}

	// Handle the simple case: git mv <source> <dest>
	if len(nonFlagArgs) == 2 {
		source := nonFlagArgs[0]
		dest := nonFlagArgs[1]

		// Check if destination exists (it should after the mv operation)
		if err := m.git.GitRun("ls-files", "--error-unmatch", dest); err != nil {
			return nil, fmt.Errorf("destination '%s' does not exist in git index, cannot undo move", dest)
		}

		return NewUndoCommand(m.git,
			fmt.Sprintf("git mv %s %s", dest, source),
			fmt.Sprintf("Move '%s' back to '%s'", dest, source),
		), nil
	}

	// Handle multiple sources into directory: git mv <source1> <source2> ... <dest_dir>
	// In this case, the last argument is the destination directory
	destDir := nonFlagArgs[len(nonFlagArgs)-1]
	sources := nonFlagArgs[:len(nonFlagArgs)-1]

	// Build the undo command to move all files back
	var undoCommands []string
	var descriptions []string

	for _, source := range sources {
		// Extract the filename from the source path
		var filename string
		if lastSlash := strings.LastIndex(source, "/"); lastSlash != -1 {
			filename = source[lastSlash+1:]
		} else {
			filename = source
		}

		// The current location should be destDir/filename
		currentPath := strings.TrimSuffix(destDir, "/") + "/" + filename

		// Check if the file exists in its new location
		if err := m.git.GitRun("ls-files", "--error-unmatch", currentPath); err != nil {
			return nil, fmt.Errorf("moved file '%s' does not exist in destination, cannot undo move", currentPath)
		}

		undoCommands = append(undoCommands, fmt.Sprintf("git mv %s %s", currentPath, source))
		descriptions = append(descriptions, fmt.Sprintf("'%s' → '%s'", currentPath, source))
	}

	// Combine all undo commands
	fullUndoCommand := strings.Join(undoCommands, " && ")
	description := fmt.Sprintf("Move files back: %s", strings.Join(descriptions, ", "))

	return NewUndoCommand(m.git, fullUndoCommand, description), nil
}
