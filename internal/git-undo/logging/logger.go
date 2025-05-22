package logging

import (
	"bufio"
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

// EntryType specifies whether to look for regular or undoed entries.
type EntryType int

const (
	// RegularEntry represents a normal, non-undoed entry.
	RegularEntry EntryType = iota
	// UndoedEntry represents an entry that has been marked as undoed.
	UndoedEntry
)

// String returns the string representation of the EntryType.
func (et EntryType) String() string {
	return [...]string{"regular", "undoed"}[et]
}

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

// lineProcessor is a function that processes each line of the log file.
// If it returns false, file reading stops.
type lineProcessor func(line string) (continueReading bool)

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
func (l *Logger) ToggleEntry(entryIdentifier string) error {
	if l.err != nil {
		return fmt.Errorf("logger is not healthy: %w", l.err)
	}

	var foundLineIdx int
	err := l.processLogFile(func(line string) bool {
		if strings.TrimSpace(strings.TrimLeft(line, "#")) == entryIdentifier {
			return false
		}

		foundLineIdx++
		return true
	})
	if err != nil {
		return err
	}

	file, err := os.OpenFile(l.logFile, os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	return toggleLine(file, foundLineIdx)
}

// GetLastEntry returns either the last regular entry or the last undoed entry based on the entryType.
// If a reference is provided in refArg, only entries from that specific reference are considered.
// If no reference is provided, uses the current reference (branch/tag/commit).
// Use "any" as refArg to match any reference.
func (l *Logger) GetLastEntry(entryType EntryType, refArg ...string) (*Entry, error) {
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
	default:
		ref = refArg[0]
	}

	var foundEntry *Entry
	err := l.processLogFile(func(line string) bool {
		// Check if this line is undoed (starts with #)
		isUndoed := strings.HasPrefix(line, "#")

		// Skip if we're looking for the wrong type
		if (entryType == RegularEntry && isUndoed) ||
			(entryType == UndoedEntry && !isUndoed) {
			return true
		}

		// Parse the log line into an Entry
		entry, err := parseLogLine(line)
		if err != nil {
			// Skip malformed lines
			return true
		}

		// Check reference if specified and not "any"
		if ref != "" && entry.Ref != ref {
			return true
		}

		// Found a matching entry!
		foundEntry = entry
		return false
	})
	if err != nil {
		return nil, err
	}

	if foundEntry == nil {
		return nil, fmt.Errorf("no %s command found in log for reference [ref=%s]", entryType, ref)
	}

	return foundEntry, nil
}

// Dump reads the log file content and writes it directly to the provided writer.
func (l *Logger) Dump(w io.Writer) error {
	if l.err != nil {
		return fmt.Errorf("logger is not healthy: %w", l.err)
	}

	// Check if file exists
	_, err := os.Stat(l.logFile)
	if os.IsNotExist(err) {
		// File doesn't exist, create an empty one
		if err := os.WriteFile(l.logFile, []byte{}, 0600); err != nil {
			return fmt.Errorf("failed to create log file: %w", err)
		}
		// Nothing to dump (file is empty)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to check log file status: %w", err)
	}

	// Open the file for reading
	file, err := os.Open(l.logFile)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Copy directly from file to writer
	_, err = io.Copy(w, file)
	if err != nil {
		return fmt.Errorf("failed to dump log file: %w", err)
	}

	return nil
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
		defer func() { _ = in.Close() }()

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

// processLogFile reads the log file line by line and calls the processor function for each line.
// This is more efficient than reading the entire file at once, especially when only
// the first few lines are needed.
func (l *Logger) processLogFile(processor lineProcessor) error {
	if l.err != nil {
		return fmt.Errorf("logger is not healthy: %w", l.err)
	}

	// Check if the file exists
	_, err := os.Stat(l.logFile)
	if os.IsNotExist(err) {
		// Create the file if it doesn't exist
		if err := os.WriteFile(l.logFile, []byte{}, 0600); err != nil {
			return fmt.Errorf("failed to create log file: %w", err)
		}

		return nil
	} else if err != nil {
		return fmt.Errorf("failed to check log file status: %w", err)
	}

	// Open the file for reading
	file, err := os.Open(l.logFile)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Create a scanner to read line by line
	scanner := bufio.NewScanner(file)

	// Process each line
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Call the processor function and check if we should continue
		if !processor(line) {
			break
		}
	}

	// Check for any scanner errors
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading log file: %w", err)
	}

	return nil
}

// parseLogLine parses a log line into an Entry.
// Format: {"d":"2025-05-16 11:02:55","ref":"main","cmd":"git commit -m 'test'"}.
func parseLogLine(line string) (*Entry, error) {
	var entry Entry
	if err := entry.UnmarshalText([]byte(line)); err != nil {
		return nil, fmt.Errorf("invalid log line format: %s", line)
	}

	return &entry, nil
}
