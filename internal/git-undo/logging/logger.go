package logging

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/amberpixels/git-undo/internal/git-undo/config"
)

// Logger manages git command logging operations.
type Logger struct {
	logDir  string
	logFile string
}

// CommandEntry represents a logged git command with its full identifier
type CommandEntry struct {
	Command    string // just the command part
	Identifier string // full line including timestamp and ref
}

// CommandType specifies whether to look for regular or undoed commands
type CommandType int

const (
	// RegularCommand represents a normal, non-undoed command
	RegularCommand CommandType = iota
	// UndoedCommand represents a command that has been marked as undoed
	UndoedCommand
)

// NewLogger creates a new Logger instance.
func NewLogger() (*Logger, error) {
	paths, err := config.GetGitPaths()
	if err != nil {
		return nil, fmt.Errorf("failed to get git paths: %w", err)
	}

	if err := config.EnsureLogDir(paths); err != nil {
		return nil, fmt.Errorf("failed to ensure log directory: %w", err)
	}

	return &Logger{
		logDir:  paths.LogDir,
		logFile: filepath.Join(paths.LogDir, "command-log.txt"),
	}, nil
}

// LogCommand logs a git command with timestamp.
func (l *Logger) LogCommand(strGitCommand string) error {
	// Skip logging git undo commands
	if strings.HasPrefix(strGitCommand, "git undo") {
		return nil
	}

	// Get current ref (branch/tag/commit)
	ref, err := l.getCurrentRef()
	if err != nil {
		// If we can't get the ref, just use "unknown"
		ref = "unknown"
	}

	entry := fmt.Sprintf("%s [%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), ref, strGitCommand)
	return l.prependLogEntry(entry)
}

// GetLogDir returns the path to the log directory.
func (l *Logger) GetLogDir() string {
	return l.logDir
}

// ToggleCommand toggles the undo state of a command by adding or removing the "#" prefix.
// The commandIdentifier should be in the format "TIMESTAMP [REF] COMMAND" (without the # prefix).
// Returns true if the command was marked as undoed, false if it was unmarked.
func (l *Logger) ToggleCommand(commandIdentifier string) (bool, error) {
	content, err := l.readLogFile()
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 {
		return false, errors.New("log file is empty")
	}

	// Find the line that matches our command identifier
	found := false
	wasMarked := false
	for i := range lines {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Check if this line matches our command identifier
		// For marked commands, we need to check without the # prefix
		if line == commandIdentifier || (strings.HasPrefix(line, "#") && line[1:] == commandIdentifier) {
			if strings.HasPrefix(line, "#") {
				// Command was marked, unmark it
				lines[i] = line[1:]
				wasMarked = false
			} else {
				// Command was not marked, mark it
				lines[i] = "#" + line
				wasMarked = true
			}
			found = true
			break
		}
	}

	if !found {
		return false, fmt.Errorf("command not found in log: %s", commandIdentifier)
	}

	// Write the modified content back to the file
	if err := os.WriteFile(l.logFile, []byte(strings.Join(lines, "\n")), 0600); err != nil {
		return false, err
	}

	return wasMarked, nil
}

// GetCommand returns either the last regular command or the last undoed command based on the commandType.
func (l *Logger) GetCommand(commandType CommandType) (*CommandEntry, error) {
	content, err := l.readLogFile()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")

	// Find the first non-empty line that matches our criteria
	for i := range lines {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		isUndoed := strings.HasPrefix(line, "#")

		// Skip if we're looking for the wrong type
		if (commandType == RegularCommand && isUndoed) ||
			(commandType == UndoedCommand && !isUndoed) {
			continue
		}

		// For undoed commands, we need to remove the # prefix
		if isUndoed {
			line = line[1:]
		}

		// Extract command part (after timestamp and ref)
		parts := strings.SplitN(line, " git ", 2)
		if len(parts) < 2 {
			continue
		}

		return &CommandEntry{
			Command:    "git " + parts[1],
			Identifier: line,
		}, nil
	}

	switch commandType {
	case RegularCommand:
		return nil, errors.New("no git command found in log")
	case UndoedCommand:
		return nil, errors.New("no undoed command found in log")
	default:
		return nil, errors.New("invalid command type")
	}
}

// getCurrentRef returns the current branch name, tag, or commit hash.
func (l *Logger) getCurrentRef() (string, error) {
	// Try to get branch name first
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output)), nil
	}

	// If not on a branch, try to get tag name
	cmd = exec.Command("git", "describe", "--tags", "--exact-match")
	output, err = cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output)), nil
	}

	// If not on a tag, get commit hash
	cmd = exec.Command("git", "rev-parse", "--short", "HEAD")
	output, err = cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current ref: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// prependLogEntry prepends a new line into the log file.
func (l *Logger) prependLogEntry(entry string) error {
	tmpFile := l.logFile + ".tmp"

	// Create a tmp file
	out, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("cannot create temporary log file: %w", err)
	}
	defer out.Close()

	// Insert our new entry line
	if _, err := out.WriteString(entry); err != nil {
		return fmt.Errorf("failed to write log entry: %w", err)
	}

	// Stream original file into the tmp file
	in, err := os.Open(l.logFile)
	switch {
	case err == nil:
		defer in.Close()
		if _, err := io.Copy(out, in); err != nil {
			return fmt.Errorf("failed to copy existing log content: %w", err)
		}
	case os.IsNotExist(err):
		// if os.Open failed because file doesn't exist, we just skip it
	default:
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Swap via rename: will remove logFile and make tmpFile our logFile
	if err := os.Rename(tmpFile, l.logFile); err != nil {
		return fmt.Errorf("failed to rename temporary log file: %w", err)
	}

	return nil
}

// readLogFile reads the content of the log file.
func (l *Logger) readLogFile() ([]byte, error) {
	if _, err := os.Stat(l.logFile); os.IsNotExist(err) {
		return nil, errors.New("no command log found")
	}

	content, err := os.ReadFile(l.logFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	return content, nil
}
