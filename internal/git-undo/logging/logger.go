package logging

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
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
	ref, err := l.git.GetCurrentGitRef()
	if err != nil {
		// If we can't get the ref, just use "unknown"
		ref = "unknown"
	}

	return l.logCommandWithDedup(strGitCommand, ref)
}

// logCommandWithDedup logs a command while preventing duplicates between shell and git hooks
func (l *Logger) logCommandWithDedup(strGitCommand, ref string) error {
	// Create a unique identifier for this command + timestamp (within 2 seconds)
	// This allows us to detect and prevent duplicates between shell and git hooks
	normalizedTime := time.Now().Truncate(2 * time.Second)
	cmdIdentifier := l.createCommandIdentifier(strGitCommand, ref, normalizedTime)

	// Debug: show normalization
	normalizedCmd := l.normalizeGitCommand(strGitCommand)
	fmt.Printf("DEBUG: Original='%s', Normalized='%s', ID='%s'\n", strGitCommand, normalizedCmd, cmdIdentifier)

	// Check if we're in a git hook (vs shell hook)
	isGitHook := l.isGitHookContext()
	fmt.Println("IS GIT HOOK", isGitHook)

	if isGitHook {
		fmt.Println("GIT HOOK: MARK LOGGED -- ", strGitCommand)
		// Git hook runs first: mark that we're logging this command
		l.markLoggedByGitHook(cmdIdentifier)
	} else {
		// Shell hook runs second: check if git hook already logged this command
		if l.wasRecentlyLoggedByGitHook(cmdIdentifier) {
			fmt.Println("GIT HOOK ALREADY LOGGED THIS")
			// Git hook already logged this, skip to avoid duplicate
			return nil
		}
		fmt.Println("SHELL HOOK: LOGGING (NO GIT HOOK FOUND) -- ", strGitCommand)
	}

	return l.prependLogEntry((&Entry{
		Timestamp: time.Now(),
		Ref:       ref,
		Command:   strGitCommand,
	}).String())
}

// createCommandIdentifier creates a short identifier for a command to detect duplicates
func (l *Logger) createCommandIdentifier(command, ref string, timestamp time.Time) string {
	// Normalize the command first to ensure equivalent commands have the same identifier
	normalizedCmd := l.normalizeGitCommand(command)

	// Create hash of normalized command + ref + truncated timestamp
	data := fmt.Sprintf("%s|%s|%d", normalizedCmd, ref, timestamp.Unix())
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])[:12] // Use first 12 characters
}

// normalizeGitCommand converts git commands to a canonical form for comparison
func (l *Logger) normalizeGitCommand(cmd string) string {
	// Parse the command
	parts := strings.Fields(cmd)
	if len(parts) < 2 || parts[0] != "git" {
		return cmd
	}

	subcommand := parts[1]
	args := parts[2:]

	switch subcommand {
	case "commit":
		return l.normalizeCommitCommand(args)
	case "merge":
		return l.normalizeMergeCommand(args)
	case "rebase":
		return l.normalizeRebaseCommand(args)
	case "cherry-pick":
		return l.normalizeCherryPickCommand(args)
	// Add other commands as needed
	default:
		// For unknown commands, just return the basic form
		return fmt.Sprintf("git %s", subcommand)
	}
}

// normalizeCommitCommand normalizes commit commands to canonical form
func (l *Logger) normalizeCommitCommand(args []string) string {
	message := ""
	amend := false

	// Parse arguments to extract key information
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-m" && i+1 < len(args):
			// Extract message, removing quotes
			message = strings.Trim(args[i+1], `"'`)
			i++ // Skip the message argument
		case arg == "--amend":
			amend = true
		case strings.HasPrefix(arg, "-m"):
			// Handle -m"message" format
			if len(arg) > 2 {
				message = strings.Trim(arg[2:], `"'`)
			}
			// Ignore other flags like --verbose, --signoff, etc.
		}
	}

	// Build normalized command
	if amend {
		return "git commit --amend"
	} else if message != "" {
		return fmt.Sprintf("git commit -m %q", message)
	} else {
		return "git commit"
	}
}

// normalizeMergeCommand normalizes merge commands to canonical form
func (l *Logger) normalizeMergeCommand(args []string) string {
	squash := false
	noFf := false
	ff := false
	branch := ""

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--squash":
			squash = true
		case "--no-ff":
			noFf = true
		case "--ff":
			ff = true
		case "--ff-only":
			ff = true
		default:
			// Assume it's a branch name if it doesn't start with -
			if !strings.HasPrefix(arg, "-") && branch == "" {
				branch = arg
			}
		}
	}

	// Build normalized command
	cmd := "git merge"
	if squash {
		cmd += " --squash"
	} else if noFf {
		cmd += " --no-ff"
	} else if ff {
		cmd += " --ff"
	}

	if branch != "" {
		cmd += " " + branch
	}

	return cmd
}

// normalizeRebaseCommand normalizes rebase commands to canonical form
func (l *Logger) normalizeRebaseCommand(args []string) string {
	interactive := false
	branch := ""

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-i", "--interactive":
			interactive = true
		default:
			if !strings.HasPrefix(arg, "-") && branch == "" {
				branch = arg
			}
		}
	}

	cmd := "git rebase"
	if interactive {
		cmd += " -i"
	}
	if branch != "" {
		cmd += " " + branch
	}

	return cmd
}

// normalizeCherryPickCommand normalizes cherry-pick commands to canonical form
func (l *Logger) normalizeCherryPickCommand(args []string) string {
	commit := ""

	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") && commit == "" {
			commit = arg
			break
		}
	}

	if commit != "" {
		return "git cherry-pick " + commit
	}
	return "git cherry-pick"
}

// isGitHookContext detects if we're running in a git hook context
// We do this by checking the call stack environment rather than relying on order
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

// wasRecentlyLoggedByShellHook checks if this command was recently logged by shell hook
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

// markLoggedByShellHook marks that this command was logged by shell hook
func (l *Logger) markLoggedByShellHook(cmdIdentifier string) {
	flagFile := filepath.Join(l.logDir, ".shell-hook-"+cmdIdentifier)

	// Create flag file
	if file, err := os.Create(flagFile); err == nil {
		file.Close()
	}

	// Clean up old flag files in background (best effort)
	go l.cleanupOldFlagFiles()
}

// cleanupOldFlagFiles removes flag files older than 30 seconds
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

// wasRecentlyLoggedByGitHook checks if this command was recently logged by git hook
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

// markLoggedByGitHook marks that this command was logged by git hook
func (l *Logger) markLoggedByGitHook(cmdIdentifier string) {
	flagFile := filepath.Join(l.logDir, ".git-hook-"+cmdIdentifier)

	// Create flag file
	if file, err := os.Create(flagFile); err == nil {
		file.Close()
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
func (l *Logger) GetLastRegularEntry(refArg ...string) (*Entry, error) {
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
		// skip undoed
		if strings.HasPrefix(line, "#") {
			return true
		}

		// Parse the log line into an Entry
		entry, err := parseLogLine(line)
		if err != nil { // TODO: warnings maybe?
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

	return foundEntry, nil
}

// GetLastEntry returns last entry for the given ref (or current ref if not specified)
// regarding of the entry type (undoed or regular).
func (l *Logger) GetLastEntry(refArg ...string) (*Entry, error) {
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
		// Parse the log line into an Entry
		entry, err := parseLogLine(line)
		if err != nil { // TODO: warnings maybe?
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
