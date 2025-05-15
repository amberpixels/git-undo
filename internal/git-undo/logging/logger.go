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

// GetLastCommand reads the last git command from the log file.
func (l *Logger) GetLastCommand() (string, error) {
	content, err := l.readLogFile()
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")

	// Find the first non-empty line that contains a git command and is not marked as undoed
	for i := range lines {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Skip lines marked as undoed
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Extract command part (after timestamp and ref)
		parts := strings.SplitN(line, " git ", 2)
		if len(parts) < 2 {
			continue
		}

		// Skip "git status" commands
		if strings.HasPrefix(parts[1], "status") {
			continue
		}

		return "git " + parts[1], nil
	}

	return "", errors.New("no git command found in log")
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

// GetLogDir returns the path to the log directory.
func (l *Logger) GetLogDir() string {
	return l.logDir
}

// MarkCommandAsUndoed marks a command as undoed by adding a "#" prefix.
func (l *Logger) MarkCommandAsUndoed() error {
	content, err := l.readLogFile()
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 {
		return errors.New("log file is empty")
	}

	// Find the first non-empty line that is not already marked as undoed
	for i := range lines {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Skip lines already marked as undoed
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Mark the line as undoed
		lines[i] = "#" + line
		break
	}

	// Write the modified content back to the file
	return os.WriteFile(l.logFile, []byte(strings.Join(lines, "\n")), 0600)
}

// GetLastUndoedCommand returns the last command that was marked as undoed.
func (l *Logger) GetLastUndoedCommand() (string, error) {
	content, err := l.readLogFile()
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")

	// Find the first non-empty line that is marked as undoed
	// This will be the most recently undoed command since we prepend new entries
	candidate := ""
	for i := range lines {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Look for lines marked as undoed
		if strings.HasPrefix(line, "#") {
			// Extract command part (after timestamp)
			parts := strings.SplitN(line[1:], " git ", 2) // Remove the # prefix
			if len(parts) < 2 {
				continue
			}
			candidate = "git " + parts[1]
			continue
		}

		if candidate == "" {
			return "", errors.New("cannot redo: there are newer undoed commands")
		}
		return candidate, nil
	}

	if candidate == "" {
		return "", errors.New("no undoed command found in log")
	}
	return candidate, nil
}

// UnmarkLastUndoedCommand removes the "#" prefix from the last undoed command.
func (l *Logger) UnmarkLastUndoedCommand() error {
	content, err := l.readLogFile()
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 {
		return errors.New("log file is empty")
	}

	// Find the first non-empty line that is marked as undoed
	for i := range lines {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Look for lines marked as undoed
		if strings.HasPrefix(line, "#") {
			// Remove the # prefix
			lines[i] = line[1:]
			break
		}
	}

	// Write the modified content back to the file
	return os.WriteFile(l.logFile, []byte(strings.Join(lines, "\n")), 0600)
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
