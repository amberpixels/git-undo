package logging

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Logger manages git command logging operations.
type Logger struct {
	logDir  string
	logFile string

	// err is nil when everything IS OK: logger is healthy, initialized OK (files exists, are accessible, etc)
	err error

	// git is a GitHelper (calling getting current ref, etc)
	git GitHelper
}

type GitHelper interface {
	GetCurrentGitRef() (string, error)
}

const (
	logEntryDateFormat = time.DateTime
	logFileDirName     = "git-undo"
	logFileName        = "commands"
)

// Entry represents a logged git command with its full identifier.
type Entry struct {
	// Timestamp is parsed timestamp of the entry.
	Timestamp time.Time
	// Ref is reference (branch/tag/commit) where the command was executed.
	Ref string
	// Command is just the command part.
	Command string

	// Undoed is true if the entry is undoed.
	Undoed bool
}

// GetIdentifier returns full command without sign of undoed state (# prefix).
func (e *Entry) GetIdentifier() string {
	return strings.TrimPrefix(e.String(), "#")
}

// String returns a human-readable representation of the entry.
// This representation goes into the log file as well.
func (e *Entry) String() string {
	text, _ := e.MarshalText()
	return string(text)
}

func (e *Entry) MarshalText() ([]byte, error) {
	entryString := fmt.Sprintf("%s|%s|%s", e.Timestamp.Format(logEntryDateFormat), e.Ref, e.Command)
	if e.Undoed {
		entryString = "#" + entryString
	}
	return []byte(entryString), nil
}

func (e *Entry) UnmarshalText(data []byte) error {
	entryString := string(data)
	if strings.HasPrefix(entryString, "#") {
		entryString = strings.TrimPrefix(entryString, "#")
		e.Undoed = true
	}

	// nMustParts = 3 for date, ref, cmd
	const nMustParts = 3

	parts := strings.SplitN(entryString, "|", nMustParts)
	if len(parts) != nMustParts {
		return fmt.Errorf("invalid log entry format: %s", entryString)
	}
	var err error
	e.Timestamp, err = time.Parse(logEntryDateFormat, parts[0])
	if err != nil {
		return fmt.Errorf("failed to parse timestamp: %w", err)
	}

	e.Ref = parts[1]
	e.Command = parts[2]

	return nil
}

// EntryType specifies whether to look for regular or undoed entries.
type EntryType int

const (
	// RegularEntry represents a normal, non-undoed entry.
	RegularEntry EntryType = iota
	// UndoedEntry represents an entry that has been marked as undoed.
	UndoedEntry
)

// NewLogger creates a new Logger instance.
func NewLogger(repoGitDir string, git GitHelper) *Logger {
	lgr := &Logger{git: git}

	// default log file path will be .git/git-undo/commands
	lgr.logDir = filepath.Join(repoGitDir, logFileDirName)
	lgr.logFile = filepath.Join(lgr.logDir, logFileName)

	if err := EnsureLogDir(lgr.logDir); err != nil {
		lgr.err = fmt.Errorf("failed to ensure log directory: %w", err)
	}

	return lgr
}

// LogCommand logs a git command with timestamp.
func (l *Logger) LogCommand(strGitCommand string) error {
	if l.err != nil {
		return fmt.Errorf("logger is not healthy: %w", l.err)
	}

	// Skip logging git undo commands
	if strings.HasPrefix(strGitCommand, "git undo") {
		return nil
	}

	// Get current ref (branch/tag/commit)
	ref, err := l.git.GetCurrentGitRef()
	if err != nil {
		// If we can't get the ref, just use "unknown"
		ref = "unknown"
	}

	return l.prependLogEntry((&Entry{
		Timestamp: time.Now(),
		Ref:       ref,
		Command:   strGitCommand,
	}).String())
}

// GetLogPath returns the path to the log file.
func (l *Logger) GetLogPath() string { return l.logFile }

// ToggleEntry toggles the undo state of an entry by adding or removing the "#" prefix.
// The entryIdentifier should be in the format "TIMESTAMP|REF|COMMAND" (without the # prefix).
// Returns true if the entry was marked as undoed, false if it was unmarked.
func (l *Logger) ToggleEntry(entryIdentifier string) (bool, error) {
	if l.err != nil {
		return false, fmt.Errorf("logger is not healthy: %w", l.err)
	}

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
		if line == entryIdentifier {
			lines[i] = "#" + line
			wasMarked = true
			found = true
			break
		} else if strings.HasPrefix(line, "#") && line[1:] == entryIdentifier {
			// Entry was marked, unmark it
			lines[i] = line[1:]
			wasMarked = false
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

// GetEntry returns either the last regular entry or the last undoed entry based on the entryType.
// If a reference is provided in refArg, only entries from that specific reference are considered.
// If no reference is provided, uses the current reference (branch/tag/commit).
// Use "any" as refArg to match any reference.
func (l *Logger) GetEntry(entryType EntryType, refArg ...string) (*Entry, error) {
	if l.err != nil {
		return nil, fmt.Errorf("logger is not healthy: %w", l.err)
	}

	// Determine which reference to use
	var ref string
	switch len(refArg) {
	case 0:
		// No ref provided, use current ref
		currentRef, err := l.git.GetCurrentGitRef()
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
			// TODO: in debug mode print the warning out
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

// prependLogEntry prepends a new line into the log file.
func (l *Logger) prependLogEntry(entry string) error {
	if l.err != nil {
		return fmt.Errorf("logger is not healthy: %w", l.err)
	}

	tmpFile := l.logFile + ".tmp"

	// Create a tmp file
	out, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("cannot create temporary log file: %w", err)
	}
	defer out.Close()

	// Insert our new entry line
	if _, err := out.WriteString(entry + "\n"); err != nil {
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
	if l.err != nil {
		return nil, fmt.Errorf("logger is not healthy: %w", l.err)
	}

	var content []byte
	if _, err := os.Stat(l.logFile); os.IsNotExist(err) {
		// let's create the file instead
		if err := os.WriteFile(l.logFile, []byte{}, 0600); err != nil {
			return nil, fmt.Errorf("failed to create log file: %w", err)
		}

		content = []byte{}
	} else {
		if content, err = os.ReadFile(l.logFile); err != nil {
			return nil, fmt.Errorf("failed to read log file: %w", err)
		}
	}

	// trim last line break
	if len(content) > 0 && content[len(content)-1] == '\n' {
		content = content[:len(content)-1]
	}

	return content, nil
}

// parseLogLine parses a log line into an Entry.
// Format: {"d":"2025-05-16 11:02:55","ref":"main","cmd":"git commit -m 'test'"}.
func parseLogLine(line string, isUndoed bool) (*Entry, error) {
	// If the line is marked as undoed, remove the # prefix
	if isUndoed {
		line = line[1:]
	}

	entry := &Entry{}
	if err := entry.UnmarshalText([]byte(line)); err != nil {
		return nil, fmt.Errorf("invalid log line format: %s", line)
	}

	return entry, nil
}
