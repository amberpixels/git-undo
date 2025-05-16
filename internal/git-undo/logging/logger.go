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

// Entry represents a logged git command with its full identifier
type Entry struct {
	Command    string    // just the command part
	Identifier string    // full line including timestamp and ref
	Timestamp  time.Time // parsed timestamp of the entry
	Ref        string    // reference (branch/tag/commit) where the command was executed
}

// EntryType specifies whether to look for regular or undoed entries
type EntryType int

const (
	// RegularEntry represents a normal, non-undoed entry
	RegularEntry EntryType = iota
	// UndoedEntry represents an entry that has been marked as undoed
	UndoedEntry
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

	// Format the log entry with the current timestamp
	entry := fmt.Sprintf("%s [%s] %s\n",
		time.Now().Format("2006-01-02 15:04:05"),
		ref,
		strGitCommand,
	)
	return l.prependLogEntry(entry)
}

// GetLogDir returns the path to the log directory.
func (l *Logger) GetLogDir() string {
	return l.logDir
}

// ToggleEntry toggles the undo state of an entry by adding or removing the "#" prefix.
// The entryIdentifier should be in the format "TIMESTAMP [REF] COMMAND" (without the # prefix).
// Returns true if the entry was marked as undoed, false if it was unmarked.
func (l *Logger) ToggleEntry(entryIdentifier string) (bool, error) {
	content, err := l.readLogFile()
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 {
		return false, errors.New("log file is empty")
	}

	// Find the line that matches our entry identifier
	found := false
	wasMarked := false
	for i := range lines {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Check if this line matches our entry identifier
		// For marked entries, we need to check without the # prefix
		if line == entryIdentifier || (strings.HasPrefix(line, "#") && line[1:] == entryIdentifier) {
			if strings.HasPrefix(line, "#") {
				// Entry was marked, unmark it
				lines[i] = line[1:]
				wasMarked = false
			} else {
				// Entry was not marked, mark it
				lines[i] = "#" + line
				wasMarked = true
			}
			found = true
			break
		}
	}

	if !found {
		return false, fmt.Errorf("entry not found in log: %s", entryIdentifier)
	}

	// Write the modified content back to the file
	if err := os.WriteFile(l.logFile, []byte(strings.Join(lines, "\n")), 0600); err != nil {
		return false, err
	}

	return wasMarked, nil
}

// parseLogLine parses a log line into an Entry.
// Format: "2025-05-16 10:37:59 [main] git commit ..."
func parseLogLine(line string, isUndoed bool) (*Entry, error) {
	// If the line is marked as undoed, remove the # prefix
	if isUndoed {
		line = line[1:]
	}

	// First split by "[" to get the timestamp part
	parts := strings.SplitN(line, "[", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid log line format: %s", line)
	}

	// Parse timestamp
	timestamp, err := time.Parse("2006-01-02 15:04:05", strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	// Split the rest by "]" to get the reference
	refParts := strings.SplitN(parts[1], "]", 2)
	if len(refParts) != 2 {
		return nil, fmt.Errorf("invalid log line format: %s", line)
	}
	ref := strings.TrimSpace(refParts[0])

	// Extract command part (after timestamp and ref)
	cmdParts := strings.SplitN(refParts[1], " git ", 2)
	if len(cmdParts) != 2 {
		return nil, fmt.Errorf("invalid log line format: %s", line)
	}

	return &Entry{
		Command:    "git " + cmdParts[1],
		Identifier: line,
		Timestamp:  timestamp,
		Ref:        ref,
	}, nil
}

// GetEntry returns either the last regular entry or the last undoed entry based on the entryType.
// If a reference is provided in refArg, only entries from that specific reference are considered.
// If no reference is provided, uses the current reference (branch/tag/commit).
// Use "any" as refArg to match any reference.
func (l *Logger) GetEntry(entryType EntryType, refArg ...string) (*Entry, error) {
	// Determine which reference to use
	var ref string
	switch len(refArg) {
	case 0:
		// No ref provided, use current ref
		currentRef, err := l.getCurrentRef()
		if err != nil {
			return nil, fmt.Errorf("failed to get current ref: %w", err)
		}
		ref = currentRef
	case 1:
		if refArg[0] == "any" {
			ref = "" // Empty ref means match any
		} else {
			ref = refArg[0]
		}
	default:
		return nil, errors.New("too many reference arguments provided")
	}

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
		if (entryType == RegularEntry && isUndoed) ||
			(entryType == UndoedEntry && !isUndoed) {
			continue
		}

		// Parse the log line into an Entry
		entry, err := parseLogLine(line, isUndoed)
		if err != nil {
			continue // Skip malformed lines
		}

		// Check reference if specified
		if ref != "" && entry.Ref != ref {
			continue // Skip entries from different references
		}

		return entry, nil
	}

	var typeStr string
	switch entryType {
	case RegularEntry:
		typeStr = "regular"
	case UndoedEntry:
		typeStr = "undoed"
	default:
		return nil, errors.New("invalid entry type")
	}

	if ref != "" {
		return nil, fmt.Errorf("no %s command found in log for reference [%s]", typeStr, ref)
	}
	return nil, fmt.Errorf("no %s command found in log", typeStr)
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
