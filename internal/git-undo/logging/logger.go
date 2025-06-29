package logging

import (
	"bufio"
	"crypto/sha1" //nolint:gosec // We're fine with this
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/amberpixels/git-undo/internal/githelpers"
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
	NotSpecifiedEntryType EntryType = iota

	// RegularEntry represents a normal, non-undoed entry.
	RegularEntry
	// UndoedEntry represents an entry that has been marked as undoed.
	UndoedEntry
)

// String returns the string representation of the EntryType.
func (et EntryType) String() string {
	return [...]string{"", "regular", "undoed"}[et]
}

type Ref string

const (
	// RefAny means when the ref (branch/tag/commit) of the line is not respected (any).
	RefAny Ref = "__ANY__"

	// RefCurrent means when the ref (branch/tag/commit) is the current one.
	RefCurrent Ref = "__CURRENT__"

	// RefUnknown means when the ref couldn't be determined. (Non-happy path).
	RefUnknown Ref = "__UNKNOWN__"

	// RefMain represents the main branch (used for convenience).
	RefMain Ref = "main"
)

func (r Ref) String() string { return string(r) }

// Entry represents a logged git command with its full identifier.
type Entry struct {
	// Timestamp is parsed timestamp of the entry.
	Timestamp time.Time
	// Ref is reference (branch/tag/commit) where the command was executed.
	Ref Ref
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

	e.Ref = Ref(parts[1])
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
		return nil
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
	var ref = RefUnknown
	refStr, err := l.git.GetCurrentGitRef()
	if err == nil {
		ref = Ref(refStr)
	}

	return l.logCommandWithDedup(strGitCommand, ref)
}

// logCommandWithDedup logs a command while preventing duplicates between shell and git hooks.
func (l *Logger) logCommandWithDedup(strGitCommand string, ref Ref) error {
	// Create a unique identifier for this command + timestamp (within 2 seconds)
	// This allows us to detect and prevent duplicates between shell and git hooks
	normalizedTime := time.Now().Truncate(2 * time.Second)
	cmdIdentifier := l.createCommandIdentifier(strGitCommand, ref, normalizedTime)

	// Check if we already handled this by other hook.
	isGitHook := l.isGitHookContext()

	if isGitHook && l.wasRecentlyLoggedByShellHook(cmdIdentifier) {
		return nil
	}
	if !isGitHook && l.wasRecentlyLoggedByGitHook(cmdIdentifier) {
		return nil
	}

	// Mark:
	if isGitHook {
		l.markLoggedByGitHook(cmdIdentifier)
	} else {
		l.markLoggedByShellHook(cmdIdentifier)
	}

	return l.prependLogEntry((&Entry{
		Timestamp: time.Now(),
		Ref:       ref,
		Command:   strGitCommand,
	}).String())
}

// createCommandIdentifier creates a short identifier for a command to detect duplicates.
func (l *Logger) createCommandIdentifier(command string, ref Ref, timestamp time.Time) string {
	// Normalize the command first to ensure equivalent commands have the same identifier
	normalizedCmd := l.normalizeGitCommand(command)

	// Create hash of normalized command + ref + truncated timestamp
	data := fmt.Sprintf("%s|%s|%d", normalizedCmd, ref, timestamp.Unix())
	hash := sha1.Sum([]byte(data))          //nolint:gosec // We're fine with this
	return hex.EncodeToString(hash[:])[:12] // Use first 12 characters
}

// normalizeGitCommand converts git commands to a canonical form for comparison.
func (l *Logger) normalizeGitCommand(cmd string) string {
	// Parse the command using the proper GitCommand parser
	gitCmd, err := githelpers.ParseGitCommand(cmd)
	if err != nil {
		// If parsing fails, return original command
		return cmd
	}

	// Attempt to normalize the command
	normalizedStr, err := gitCmd.NormalizedString()
	if err != nil {
		// If normalization fails (e.g., command not supported), return original
		return cmd
	}

	// NormalizedString() already includes "git" prefix via String() method
	return normalizedStr
}

// isGitHookContext detects if we're running in a git hook context
// We do this by checking the call stack environment rather than relying on order.
func (l *Logger) isGitHookContext() bool {
	// Method 1: Check if we're called from the git hook script
	// Our git hook script sets a special marker
	if marker := os.Getenv("GIT_UNDO_GIT_HOOK_MARKER"); marker == "1" {
		return true
	}

	// Method 2: Heuristic - check if GIT_DIR is set (git hooks usually have this)
	// This is less reliable but serves as fallback
	if _, hasGitDir := os.LookupEnv("GIT_DIR"); hasGitDir {
		// Additionally check that we're not in shell hook context
		if os.Getenv("GIT_UNDO_INTERNAL_HOOK") == "1" {
			// This could be either shell or git hook, need to differentiate
			// Check if common git hook environment variables are set
			hookName := os.Getenv("GIT_HOOK_NAME")
			return hookName != ""
		}
	}

	return false
}

// wasRecentlyLoggedByShellHook checks if this command was recently logged by shell hook.
func (l *Logger) wasRecentlyLoggedByShellHook(cmdIdentifier string) bool {
	flagFile := filepath.Join(l.logDir, ".shell-hook-"+cmdIdentifier)

	// Check if flag file exists and is recent (within last 10 seconds)
	if stat, err := os.Stat(flagFile); err == nil {
		age := time.Since(stat.ModTime())
		if age < 10*time.Second {
			return true
		}
		// Clean up old flag file
		_ = os.Remove(flagFile)
	}

	return false
}

// markLoggedByShellHook marks that this command was logged by shell hook.
func (l *Logger) markLoggedByShellHook(cmdIdentifier string) {
	flagFile := filepath.Join(l.logDir, ".shell-hook-"+cmdIdentifier)

	// Create flag file
	if file, err := os.Create(flagFile); err == nil {
		_ = file.Close()
	}

	// Clean up old flag files in background (best effort)
	go l.cleanupOldFlagFiles()
}

// cleanupOldFlagFiles removes flag files older than 30 seconds.
func (l *Logger) cleanupOldFlagFiles() {
	entries, err := os.ReadDir(l.logDir)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-30 * time.Second)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".shell-hook-") && !strings.HasPrefix(entry.Name(), ".git-hook-") {
			continue
		}

		filePath := filepath.Join(l.logDir, entry.Name())
		if stat, err := os.Stat(filePath); err == nil && stat.ModTime().Before(cutoff) {
			_ = os.Remove(filePath)
		}
	}
}

// wasRecentlyLoggedByGitHook checks if this command was recently logged by git hook.
func (l *Logger) wasRecentlyLoggedByGitHook(cmdIdentifier string) bool {
	flagFile := filepath.Join(l.logDir, ".git-hook-"+cmdIdentifier)

	// Check if flag file exists and is recent (within last 10 seconds)
	if stat, err := os.Stat(flagFile); err == nil {
		age := time.Since(stat.ModTime())
		if age < 10*time.Second {
			return true
		}
		// Clean up old flag file
		_ = os.Remove(flagFile)
	}

	return false
}

// markLoggedByGitHook marks that this command was logged by git hook.
func (l *Logger) markLoggedByGitHook(cmdIdentifier string) {
	flagFile := filepath.Join(l.logDir, ".git-hook-"+cmdIdentifier)

	// Create flag file
	if file, err := os.Create(flagFile); err == nil {
		_ = file.Close()
	}

	// Clean up old flag files in background (best effort)
	go l.cleanupOldFlagFiles()
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

// GetLastRegularEntry returns last regular entry (ignoring undoed ones)
// for the given ref (or current ref if not specified).
func (l *Logger) GetLastRegularEntry(refArg ...Ref) (*Entry, error) {
	if l.err != nil {
		return nil, fmt.Errorf("logger is not healthy: %w", l.err)
	}
	ref := l.resolveRef(refArg...)

	var foundEntry *Entry
	err := l.processLogFile(func(line string) bool {
		// skip undoed
		if strings.HasPrefix(line, "#") {
			return true
		}

		// Parse the log line into an Entry
		entry, err := parseLogLine(line)
		if err != nil { // TODO: Logger.lgr should display warnings in Verbose mode here
			return true
		}

		if !l.matchRef(entry.Ref, ref) {
			return true
		}

		// Found a matching entry!
		foundEntry = entry
		return false
	})
	if err != nil {
		return nil, err
	}

	return foundEntry, nil
}

// GetLastEntry returns last entry for the given ref (or current ref if not specified)
// regarding of the entry type (undoed or regular).
func (l *Logger) GetLastEntry(refArg ...Ref) (*Entry, error) {
	if l.err != nil {
		return nil, fmt.Errorf("logger is not healthy: %w", l.err)
	}

	ref := l.resolveRef(refArg...)

	var foundEntry *Entry
	err := l.processLogFile(func(line string) bool {
		// Parse the log line into an Entry
		entry, err := parseLogLine(line)
		if err != nil { // TODO: warnings maybe?
			return true
		}

		if !l.matchRef(entry.Ref, ref) {
			return true
		}

		// Found a matching entry!
		foundEntry = entry
		return false
	})
	if err != nil {
		return nil, err
	}

	return foundEntry, nil
}

// GetLastCheckoutSwitchEntry returns the last checkout or switch command entry
// for the given ref (or current ref if not specified).
func (l *Logger) GetLastCheckoutSwitchEntry(refArg ...Ref) (*Entry, error) {
	if l.err != nil {
		return nil, fmt.Errorf("logger is not healthy: %w", l.err)
	}

	ref := l.resolveRef(refArg...)

	var foundEntry *Entry
	err := l.processLogFile(func(line string) bool {
		// skip undoed
		if strings.HasPrefix(line, "#") {
			return true
		}

		// Parse the log line into an Entry
		entry, err := parseLogLine(line)
		if err != nil { // TODO: warnings maybe?
			return true
		}
		if !l.matchRef(entry.Ref, ref) {
			return true
		}

		// Check if this is a checkout or switch command
		if !isCheckoutOrSwitchCommand(entry.Command) {
			return true
		}

		// Found a matching entry!
		foundEntry = entry
		return false
	})
	if err != nil {
		return nil, err
	}

	return foundEntry, nil
}

// GetLastCheckoutSwitchEntryForToggle returns the last checkout or switch command entry
// for git-back, including undoed entries. This allows git-back to toggle back and forth.
func (l *Logger) GetLastCheckoutSwitchEntryForToggle(refArg ...Ref) (*Entry, error) {
	if l.err != nil {
		return nil, fmt.Errorf("logger is not healthy: %w", l.err)
	}

	ref := l.resolveRef(refArg...)

	var foundEntry *Entry
	err := l.processLogFile(func(line string) bool {
		// Parse the log line into an Entry (including undoed entries)
		entry, err := parseLogLine(line)
		if err != nil { // TODO: warnings maybe?
			return true
		}
		if !l.matchRef(entry.Ref, ref) {
			return true
		}

		// Check if this is a checkout or switch command
		if !isCheckoutOrSwitchCommand(entry.Command) {
			return true
		}

		// Found a matching entry (even if undoed)!
		foundEntry = entry
		return false
	})
	if err != nil {
		return nil, err
	}

	return foundEntry, nil
}

// isCheckoutOrSwitchCommand checks if a command is a git checkout or git switch command.
func isCheckoutOrSwitchCommand(command string) bool {
	// Parse the command to check its type
	gitCmd, err := githelpers.ParseGitCommand(command)
	if err != nil {
		return false
	}

	return gitCmd.Name == "checkout" || gitCmd.Name == "switch"
}

// Dump reads the log file content and writes it directly to the provided writer.
func (l *Logger) Dump(w io.Writer) error {
	if l.err != nil {
		return fmt.Errorf("logger is not healthy: %w", l.err)
	}

	file, err := l.getFile()
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to dump (file is empty)
		}
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Copy directly from file to writer
	if _, err = io.Copy(w, file); err != nil {
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

	in, err := l.getFile()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	// if file exists, stream original file into the tmp file
	if in != nil {
		// Stream original file into the tmp file
		if _, err := io.Copy(out, in); err != nil {
			return fmt.Errorf("failed to copy existing log content: %w", err)
		}
		defer func() { _ = in.Close() }()
	}

	// Swap via rename: will remove logFile and make tmpFile our logFile
	if err := os.Rename(tmpFile, l.logFile); err != nil {
		return fmt.Errorf("failed to rename temporary log file: %w", err)
	}

	return nil
}

// resolveRef resolves the ref argument to a Ref.
func (l *Logger) resolveRef(refArg ...Ref) Ref {
	if len(refArg) == 0 || refArg[0] == RefCurrent {
		// No ref provided, use current ref
		currentRef, err := l.git.GetCurrentGitRef()
		if err != nil {
			return RefAny
		}
		return Ref(currentRef)
	}

	return refArg[0]
}

// matchRef checks if a line ref matches a target ref.
func (l *Logger) matchRef(lineRef, targetRef Ref) bool {
	if targetRef == RefAny {
		return true
	}
	if targetRef == RefCurrent {
		panic("matchRef MUST be called after RefCurrent is resolved")
	}
	if targetRef == RefUnknown {
		panic("matchRef MUST be not be called with RefUnknown")
	}

	return lineRef == targetRef
}

// processLogFile reads the log file line by line and calls the processor function for each line.
// This is more efficient than reading the entire file at once, especially when only
// the first few lines are needed.
func (l *Logger) processLogFile(processor lineProcessor) error {
	if l.err != nil {
		return fmt.Errorf("logger is not healthy: %w", l.err)
	}

	// Check if the file exists
	file, err := l.getFile()
	if err != nil {
		if os.IsNotExist(err) {
			return nil // will log error OR nil if file doesn't exist
		}
		return err
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

// getFile returns the os.File for the log file.
// It opens it for reading. If file doesn't exist it creates it (but still returns os.ErrNotExist).
// User is responsible for closing the file.
func (l *Logger) getFile() (*os.File, error) {
	// Check if the file exists
	_, err := os.Stat(l.logFile)
	if os.IsNotExist(err) {
		// Create the file if it doesn't exist
		// TODO: should we stick to os.Create() instead?
		if err := os.WriteFile(l.logFile, []byte{}, 0600); err != nil {
			return nil, fmt.Errorf("failed to create log file: %w", err)
		}

		return nil, err
	} else if err != nil {
		return nil, fmt.Errorf("failed to check log file status: %w", err)
	}

	// Open the file for reading or writing
	return os.OpenFile(l.logFile, os.O_RDONLY, 0600)
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
