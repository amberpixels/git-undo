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
)

// Logger manages git command logging operations.
type Logger struct {
	logDir    string
	logFile   string
	isVerbose bool
}

// NewLogger creates a new Logger instance.
func NewLogger(isVerbose bool) (*Logger, error) {
	// Find the git directory
	gitDirOut, err := exec.Command("git", "rev-parse", "--git-dir").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to find git dir: %w", err)
	}
	gitDir := strings.TrimSpace(string(gitDirOut))

	// Create undo-logs directory
	logDir := filepath.Join(gitDir, "undo-logs")
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return nil, fmt.Errorf("cannot create log dir: %w", err)
	}

	return &Logger{
		logDir:    logDir,
		logFile:   filepath.Join(logDir, "command-log.txt"),
		isVerbose: isVerbose,
	}, nil
}

// LogCommand logs a git command with timestamp.
func (l *Logger) LogCommand(strGitCommand string) error {
	entry := fmt.Sprintf("%s %s\n", time.Now().Format("2006-01-02 15:04:05"), strGitCommand)
	return l.prependLogEntry(entry)
}

// GetLastCommand reads the last git command from the log file.
func (l *Logger) GetLastCommand() (string, error) {
	// Check if log file exists
	if _, err := os.Stat(l.logFile); os.IsNotExist(err) {
		return "", errors.New("no command log found. Run some git commands first")
	}

	content, err := os.ReadFile(l.logFile)
	if err != nil {
		return "", fmt.Errorf("failed to read log file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Find the first non-empty line that contains a git command
	for i := range lines {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Extract command part (after timestamp)
		parts := strings.SplitN(line, " git ", 2)
		if len(parts) < 2 {
			continue
		}

		// Skip "git undo" commands
		if strings.HasPrefix(parts[1], "undo") {
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
		return fmt.Errorf("cannot create tmp log: %w", err)
	}
	defer out.Close()

	// Insert our new entry line
	if _, err := out.WriteString(entry); err != nil {
		return fmt.Errorf("write entry failed: %w", err)
	}

	// Stream original file into the tmp file
	in, err := os.Open(l.logFile)
	switch {
	case err == nil:
		defer in.Close()
		if _, err := io.Copy(out, in); err != nil {
			return fmt.Errorf("stream old log failed: %w", err)
		}
	case os.IsNotExist(err):
		// if os.Open failed because file doesn't exist, we just skip it
	default:
		return fmt.Errorf("could not open log file: %w", err)
	}

	// Swap via rename: will remove logFile and make tmpFile our logFile
	if err := os.Rename(tmpFile, l.logFile); err != nil {
		return fmt.Errorf("rename tmp log failed: %w", err)
	}

	if l.isVerbose {
		fmt.Fprintf(os.Stderr, "Logged command: %s", entry)
	}

	return nil
}

// GetLogDir returns the path to the log directory.
func (l *Logger) GetLogDir() string {
	return l.logDir
}
