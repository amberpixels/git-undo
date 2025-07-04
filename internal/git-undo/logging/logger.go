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

	// err is nil when everything IS OK:
	// logger is healthy, initialized OK (files exists, are accessible, etc)
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
	// NavigationEntry represents a navigation command entry.
	NavigationEntry
)

// String returns the string representation of the EntryType.
func (et EntryType) String() string {
	return [...]string{"", "regular", "undoed", "navigation"}[et]
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

	// IsNavigation is true if this is a navigation command (checkout/switch).
	IsNavigation bool
}

// GetIdentifier uses String() representation as the identifier itself
// But without prefix sign (so undoed command are still found)
func (e *Entry) GetIdentifier() string {
	return strings.TrimLeft(
		e.String(), "+-",
	)
}

// String returns a human-readable representation of the entry.
// This representation goes into the log file as well.
func (e *Entry) String() string {
	text, _ := e.MarshalText()
	return string(text)
}

func (e *Entry) MarshalText() ([]byte, error) {
	// Determine prefix based on navigation type and undo status
	prefixLetter := "M" // M for `modified` as the regular entry type
	if e.IsNavigation {
		prefixLetter = "N"
	}
	prefixSign := "+"
	if e.Undoed {
		prefixSign = "-"
	}
	prefix := prefixSign + prefixLetter + " "

	entryString := fmt.Sprintf("%s%s|%s|%s", prefix, e.Timestamp.Format(logEntryDateFormat), e.Ref, e.Command)
	return []byte(entryString), nil
}

func (e *Entry) UnmarshalText(data []byte) error {
	entryString := string(data)

	if strings.HasPrefix(entryString, "+") {
		e.Undoed = false
	} else if strings.HasPrefix(entryString, "-") {
		e.Undoed = true
	} else {
		return fmt.Errorf("invalid syntax line: entry must start with +/-, not [%s]", string(entryString[0]))
	}

	entryString = strings.TrimLeft(entryString, "+-")
	if strings.HasPrefix(entryString, "M") {
		e.IsNavigation = false
	} else if strings.HasPrefix(entryString, "N") {
		e.IsNavigation = true
	} else {
		return fmt.Errorf("invalid syntax line: entry must have M/N prefix, not [%s]", string(entryString[0]))
	}

	entryString = strings.TrimLeft(entryString, "MN")
	entryString = strings.TrimSpace(entryString)

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

// NewLogger creates a new Logger instance.
func NewLogger(repoGitDir string, git GitHelper) *Logger {
	lgr := &Logger{git: git}

	// default log file path will be .git/git-undo/commands
	lgr.logDir = filepath.Join(repoGitDir, logFileDirName)
	lgr.logFile = filepath.Join(lgr.logDir, logFileName)

	if err := EnsureLogDir(lgr.logDir); err != nil {
		return nil
	}

	// Check if we need to migrate/truncate old format
	if err := lgr.migrateOldFormatIfNeeded(); err != nil {
		// If migration fails, we continue but the logger might have issues
		// TODO: Add verbose logging here (and remove panic)
		panic("should not happen " + err.Error())
	}

	return lgr
}

// migrateOldFormatIfNeeded checks if the log file has old format entries and truncates it if needed.
func (l *Logger) migrateOldFormatIfNeeded() error {
	// Check if the log file exists
	_, err := os.Stat(l.logFile)
	if os.IsNotExist(err) {
		// No log file exists, nothing to migrate
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check log file: %w", err)
	}

	// Read the first few lines to check format
	file, err := os.Open(l.logFile)
	if err != nil {
		return fmt.Errorf("failed to open log file for migration check: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	hasOldFormat := false

	// Check first 10 lines or until we find new format
	for scanner.Scan() && lineCount < 10 {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		lineCount++

		// Check if this line uses new format (+M, -M, +N, -N)
		if strings.HasPrefix(line, "+M ") || strings.HasPrefix(line, "-M ") ||
			strings.HasPrefix(line, "+N ") || strings.HasPrefix(line, "-N ") {
			// Found new format, no migration needed
			return nil
		}

		// Check if this line uses old format (N prefix, # prefix, or no prefix)
		if strings.HasPrefix(line, "N ") || strings.HasPrefix(line, "#") ||
			(!strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "-")) {
			hasOldFormat = true
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading log file for migration: %w", err)
	}

	// If we found old format, truncate the file
	if hasOldFormat && lineCount > 0 {
		if err := os.Truncate(l.logFile, 0); err != nil {
			return fmt.Errorf("failed to truncate old format log file: %w", err)
		}
	}

	return nil
}

// LogCommand logs a git command with timestamp and handles branch-aware logging.
func (l *Logger) LogCommand(strGitCommand string) error {
	if l.err != nil {
		return fmt.Errorf("logger is not healthy: %w", l.err)
	}

	// Parse and check if command should be logged
	gitCmd, err := githelpers.ParseGitCommand(strGitCommand)
	if err != nil {
		// If we can't parse it, skip logging to be safe
		return nil
	}
	if !ShouldBeLogged(gitCmd) {
		return nil
	}

	// Get current ref (branch/tag/commit)
	var ref = RefUnknown
	refStr, err := l.git.GetCurrentGitRef()
	if err == nil {
		ref = Ref(refStr)
	}

	// Handle branch-aware logging for mutation commands
	if !l.IsNavigationCommand(strGitCommand) {
		// Check if we have consecutive undone commands
		undoneCount, err := l.CountConsecutiveUndoneCommands(ref)
		if err == nil && undoneCount > 0 {
			// We're branching - truncate undone mutation commands
			if err := l.TruncateToCurrentBranch(ref); err != nil {
				// Log the error but don't fail the operation
				// TODO: Add verbose logging here
				_ = err
			}
		}
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

	// Create entry with proper navigation flag
	isNav := l.IsNavigationCommand(strGitCommand)
	entry := &Entry{
		Timestamp:    time.Now(),
		Ref:          ref,
		Command:      strGitCommand,
		Undoed:       false,
		IsNavigation: isNav,
	}

	return l.prependLogEntry(entry.String())
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
	err := l.ProcessLogFile(func(line string) bool {
		// Parse the entry and get its identifier
		entry, err := ParseLogLine(line)
		if err != nil {
			foundLineIdx++
			return true
		}

		if entry.GetIdentifier() == entryIdentifier {
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
// For git-undo, this skips navigation commands (N prefixed).
func (l *Logger) GetLastRegularEntry(refArg ...Ref) (*Entry, error) {
	if l.err != nil {
		return nil, fmt.Errorf("logger is not healthy: %w", l.err)
	}
	ref := l.resolveRef(refArg...)

	var foundEntry *Entry
	err := l.ProcessLogFile(func(line string) bool {
		// Parse the log line into an Entry
		entry, err := ParseLogLine(line)
		if err != nil { // TODO: Logger.lgr should display warnings in Verbose mode here
			return true
		}

		// Skip navigation commands - git-undo doesn't process these
		if entry.IsNavigation {
			return true
		}

		// Skip undoed entries
		if entry.Undoed {
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

// GetLastUndoedEntry returns the last undoed entry for the given ref (or current ref if not specified).
// This is used for redo functionality to find the most recent undoed command to re-execute.
// For git-undo, this skips navigation commands (N prefixed).
func (l *Logger) GetLastUndoedEntry(refArg ...Ref) (*Entry, error) {
	if l.err != nil {
		return nil, fmt.Errorf("logger is not healthy: %w", l.err)
	}
	ref := l.resolveRef(refArg...)

	var foundEntry *Entry
	err := l.ProcessLogFile(func(line string) bool {
		// Parse the log line into an Entry
		entry, err := ParseLogLine(line)
		if err != nil { // TODO: Logger.lgr should display warnings in Verbose mode here
			return true
		}

		// Skip navigation commands - git-undo doesn't process these
		if entry.IsNavigation {
			return true
		}

		// Only process undoed entries
		if !entry.Undoed {
			return true
		}

		if !l.matchRef(entry.Ref, ref) {
			return true
		}

		// Found a matching undoed entry!
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
// This handles both navigation commands (N prefixed) and mutation commands.
func (l *Logger) GetLastEntry(refArg ...Ref) (*Entry, error) {
	if l.err != nil {
		return nil, fmt.Errorf("logger is not healthy: %w", l.err)
	}

	ref := l.resolveRef(refArg...)

	var foundEntry *Entry
	err := l.ProcessLogFile(func(line string) bool {
		// Parse the log line into an Entry
		entry, err := ParseLogLine(line)
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
// This method finds NON-UNDOED navigation commands for git-back.
func (l *Logger) GetLastCheckoutSwitchEntry(refArg ...Ref) (*Entry, error) {
	if l.err != nil {
		return nil, fmt.Errorf("logger is not healthy: %w", l.err)
	}

	ref := l.resolveRef(refArg...)

	var foundEntry *Entry
	err := l.ProcessLogFile(func(line string) bool {
		// Parse the log line into an Entry
		entry, err := ParseLogLine(line)
		if err != nil { // TODO: warnings maybe?
			return true
		}
		if !l.matchRef(entry.Ref, ref) {
			return true
		}

		// Skip undoed entries
		if entry.Undoed {
			return true
		}

		// Only process navigation commands
		if !entry.IsNavigation {
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
// This method finds ANY navigation command (including undoed ones) for toggle behavior.
func (l *Logger) GetLastCheckoutSwitchEntryForToggle(refArg ...Ref) (*Entry, error) {
	if l.err != nil {
		return nil, fmt.Errorf("logger is not healthy: %w", l.err)
	}

	ref := l.resolveRef(refArg...)

	var foundEntry *Entry
	err := l.ProcessLogFile(func(line string) bool {
		// Parse the log line into an Entry (including undoed entries)
		entry, err := ParseLogLine(line)
		if err != nil { // TODO: warnings maybe?
			return true
		}
		if !l.matchRef(entry.Ref, ref) {
			return true
		}

		// Only process navigation commands
		if !entry.IsNavigation {
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

// IsNavigationCommand checks if a command is a navigation command (checkout, switch, etc.).
func (l *Logger) IsNavigationCommand(command string) bool {
	return isCheckoutOrSwitchCommand(command)
}

// CountConsecutiveUndoneCommands counts consecutive undone mutation commands from the top of the log.
// It ignores navigation commands (N prefixed) and only counts mutation commands.
func (l *Logger) CountConsecutiveUndoneCommands(refArg ...Ref) (int, error) {
	if l.err != nil {
		return 0, fmt.Errorf("logger is not healthy: %w", l.err)
	}

	ref := l.resolveRef(refArg...)
	count := 0

	err := l.ProcessLogFile(func(line string) bool {
		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			return true
		}

		// Parse the log line into an Entry
		entry, err := ParseLogLine(line)
		if err != nil {
			return true // Skip malformed lines
		}

		// Skip navigation commands
		if entry.IsNavigation {
			return true
		}

		// Check if this entry matches our target ref
		if !l.matchRef(entry.Ref, ref) {
			return true
		}

		// If this is an undone mutation command, count it
		if entry.Undoed {
			count++
			return true
		}

		// If we hit a non-undone mutation command, stop counting
		return false
	})

	if err != nil {
		return 0, err
	}

	return count, nil
}

// TruncateToCurrentBranch removes undone mutation commands from the log while preserving
// all navigation commands. This implements the branch-aware behavior.
func (l *Logger) TruncateToCurrentBranch(refArg ...Ref) error {
	if l.err != nil {
		return fmt.Errorf("logger is not healthy: %w", l.err)
	}

	ref := l.resolveRef(refArg...)

	// Read all lines and filter out undone mutation commands for the target ref
	var filteredLines []string
	err := l.ProcessLogFile(func(line string) bool {
		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			return true
		}

		// Parse the log line into an Entry
		entry, err := ParseLogLine(line)
		if err != nil {
			// Keep malformed lines as-is for safety
			filteredLines = append(filteredLines, line)
			return true
		}

		// Always preserve navigation commands
		if entry.IsNavigation {
			filteredLines = append(filteredLines, line)
			return true
		}

		// If this entry doesn't match our target ref, keep it
		if !l.matchRef(entry.Ref, ref) {
			filteredLines = append(filteredLines, line)
			return true
		}

		// For entries matching our ref: keep only non-undone mutation commands
		if !entry.Undoed {
			filteredLines = append(filteredLines, line)
		}
		// Skip undone mutation commands (they get truncated)

		return true
	})

	if err != nil {
		return err
	}

	// Write the filtered lines back to the log file
	return l.rewriteLogFile(filteredLines)
}

// rewriteLogFile completely rewrites the log file with the provided lines.
func (l *Logger) rewriteLogFile(lines []string) error {
	tmpFile := l.logFile + ".tmp"

	// Create a temp file
	out, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("cannot create temporary log file: %w", err)
	}
	defer out.Close()

	// Write all lines to the temp file
	for _, line := range lines {
		if _, err := out.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("failed to write log line: %w", err)
		}
	}

	// Replace the original file
	if err := os.Rename(tmpFile, l.logFile); err != nil {
		return fmt.Errorf("failed to rename temporary log file: %w", err)
	}

	return nil
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

// ProcessLogFile reads the log file line by line and calls the processor function for each line.
// This is more efficient than reading the entire file at once, especially when only
// the first few lines are needed.
func (l *Logger) ProcessLogFile(processor func(line string) bool) error {
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

// ParseLogLine parses a log line into an Entry.
func ParseLogLine(line string) (*Entry, error) {
	var entry Entry
	if err := entry.UnmarshalText([]byte(line)); err != nil {
		return nil, fmt.Errorf("invalid log line format: %s", line)
	}

	return &entry, nil
}

// ShouldBeLogged returns true if the command should be logged.
func ShouldBeLogged(gitCmd *githelpers.GitCommand) bool {
	// Internal commands (git undo and git back) should never be logged
	if gitCmd.Name == githelpers.CustomCommandBack || gitCmd.Name == githelpers.CustomCommandUndo {
		return false
	}

	// Mutating and navigating commands are logged
	return gitCmd.BehaviorType == githelpers.Mutating || gitCmd.BehaviorType == githelpers.Navigating
}
