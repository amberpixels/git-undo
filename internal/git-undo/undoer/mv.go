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

// GetUndoCommands returns the commands that would undo the mv operation.
func (m *MvUndoer) GetUndoCommands() ([]*UndoCommand, error) {
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

		return []*UndoCommand{
			NewUndoCommand(m.git,
				fmt.Sprintf("git mv %s %s", dest, source),
				fmt.Sprintf("Move '%s' back to '%s'", dest, source),
			),
		}, nil
	}

	// Handle multiple sources into directory: git mv <source1> <source2> ... <dest_dir>
	// In this case, the last argument is the destination directory
	destDir := nonFlagArgs[len(nonFlagArgs)-1]
	sources := nonFlagArgs[:len(nonFlagArgs)-1]

	// Build separate undo commands for each file
	var undoCommands []*UndoCommand

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

		// Create individual undo command for this file
		undoCmd := NewUndoCommand(m.git,
			fmt.Sprintf("git mv %s %s", currentPath, source),
			fmt.Sprintf("Move '%s' back to '%s'", currentPath, source),
		)
		undoCommands = append(undoCommands, undoCmd)
	}

	return undoCommands, nil
}
