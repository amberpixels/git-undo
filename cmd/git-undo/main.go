package main

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

func main() {

	// Parse command-line flags
	verbose := isVerbose()
	if verbose {
		fmt.Fprintf(os.Stderr, "git-undo process called\n")
	}

	args := os.Args[1:]

	if idx, hookArg := isMatchingArg(args, func(arg string) bool {
		return strings.HasPrefix(arg, "--hook")
	}); idx >= 0 {
		if verbose {
			fmt.Fprintf(os.Stderr, "hook: start\n")
		}

		val, ok := os.LookupEnv("GIT_UNDO_INTERNAL_HOOK")
		if !ok || val != "1" {
			if verbose {
				fmt.Fprintf(os.Stderr, "Unsuccessfull hook attempt")
			}
			// Hook MUST NOT be called by user, but by our zsh script
			os.Exit(1)
		}

		arg := hookArg

		hooked := strings.TrimSpace(strings.TrimPrefix(arg, "--hook"))
		hooked = strings.TrimSpace(strings.TrimPrefix(hooked, "="))

		if isReadOnlyGitCommand(hooked) {
			if verbose {
				fmt.Fprintf(os.Stderr, "hook: skipping %q\n", hooked)
			}
			os.Exit(0)
		}

		if err := logGitCommand(hooked); err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "hook failed: %s", err)
			}
			os.Exit(1)
		}

		if verbose {
			fmt.Fprintf(os.Stderr, "hook: prepended %q\n", hooked)
		}
		os.Exit(0)
	}

	// Get the repository-specific log file path
	logFilePath, err := getGitLogFilePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if verbose {
		fmt.Printf("Using command log: %s\n", logFilePath)
	}

	// Read the last git command from the log file
	lastCmd, err := getLastGitCommand(logFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to get last git command: %v\n", err)
		os.Exit(1)
	}

	if verbose {
		fmt.Printf("Last git command: %s\n", lastCmd)
	}

	// Parse the command to determine what action to take
	cmdParts := strings.Fields(lastCmd)
	if len(cmdParts) < 2 {
		fmt.Fprintf(os.Stderr, "Error: Invalid git command format: %s\n", lastCmd)
		os.Exit(1)
	}

	// Extract the git subcommand (e.g., "commit", "add", "branch")
	subCmd := cmdParts[1]

	// Handle different git commands
	success := false
	switch subCmd {
	case "commit":
		success = undoCommit(verbose)
	case "add":
		success = undoAdd(cmdParts[2:], verbose)
	case "branch":
		const nCmdParts = 3 // git branch X
		if len(cmdParts) >= nCmdParts {
			success = undoBranch(cmdParts[2], verbose)
		}
	default:
		fmt.Printf("Cannot undo git command: %s\n", subCmd)
		fmt.Println("Supported commands: commit, add, branch")
		os.Exit(1)
	}

	if success {
		fmt.Printf("Successfully undid: %s\n", lastCmd)
	} else {
		fmt.Fprintf(os.Stderr, "Failed to undo: %s\n", lastCmd)
		os.Exit(1)
	}
}

func isVerbose() bool {
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "--verbose" {
			return true
		}
	}
	return false
}

func isMatchingArg(args []string, cb func(arg string) bool) (int, string) {
	for idx, arg := range args {
		if cb(arg) {
			return idx, arg
		}
	}

	return -1, ""
}

func logGitCommand(strGitCommand string) error {
	// 1) find the git dir
	gitDirOut, err := exec.Command("git", "rev-parse", "--git-dir").Output()
	if err != nil {
		return fmt.Errorf("failed to find git dir: %w", err)
	}
	gitDir := strings.TrimSpace(string(gitDirOut))

	// 2) ensure undo-logs exists
	logDir := filepath.Join(gitDir, "undo-logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("cannot create log dir: %w", err)
	}

	// 3) append the timestamped entry
	logFile := filepath.Join(logDir, "command-log.txt")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("cannot open log file: %w", err)
	}
	defer f.Close()

	entry := fmt.Sprintf("%s %s\n", time.Now().Format("2006-01-02 15:04:05"), strGitCommand)
	if err := prependLogEntry(logFile, entry); err != nil {
		return fmt.Errorf("can not prepend entry: %w", err)
	}

	return nil
}

// prependLogEntry prepends a new line into a logFile
// it's done as tmpFile=[newLine logFile...] -renaming-> logFile
func prependLogEntry(logFile, entry string) error {
	tmpFile := logFile + ".tmp"

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
	in, err := os.Open(logFile)
	switch {
	case err == nil:
		defer in.Close()
		if _, err := io.Copy(out, in); err != nil {
			return fmt.Errorf("stream old log failed: %w", err)
		}
	case errors.Is(err, os.ErrNotExist):
	// if os.Open failed because file doesn't exist, we just skip it
	default:
		return fmt.Errorf("could not open log file: %w", err)

	}

	// Swap via rename: will remove logFile and make tmpFile our logFile
	if err := os.Rename(tmpFile, logFile); err != nil {
		return fmt.Errorf("rename tmp log failed: %w", err)
	}

	return nil
}

// getGitLogFilePath returns the path to the git command log for the current repository.
func getGitLogFilePath() (string, error) {
	// Get the git repository root directory
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git repository root: %w", err)
	}

	// Get the git directory (usually .git, but could be elsewhere in worktrees)
	gitDirCmd := exec.Command("git", "rev-parse", "--git-dir")
	gitDirOutput, err := gitDirCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git directory: %w", err)
	}

	gitDir := strings.TrimSpace(string(gitDirOutput))

	// If gitDir is not an absolute path, make it absolute relative to the repo root
	if !filepath.IsAbs(gitDir) {
		repoRoot := strings.TrimSpace(string(output))
		gitDir = filepath.Join(repoRoot, gitDir)
	}

	// Create our custom log directory inside the git directory
	logDir := filepath.Join(gitDir, "undo-logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create log directory: %w", err)
	}

	return filepath.Join(logDir, "command-log.txt"), nil
}

// getLastGitCommand reads the last git command from the log file.
func getLastGitCommand(logFilePath string) (string, error) {
	// Check if log file exists
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		return "", errors.New("no command log found. Run some git commands first")
	}

	content, err := os.ReadFile(logFilePath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")

	// Find the first non-empty line that contains a git command
	for i := 0; i < len(lines); i++ {
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

// undoCommit reverts the last commit.
func undoCommit(verbose bool) bool {
	if verbose {
		fmt.Println("Undoing last commit with 'git reset --soft HEAD~1'")
	}

	cmd := exec.Command("git", "reset", "--soft", "HEAD~1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run() == nil
}

// undoAdd unstages files that were added.
func undoAdd(files []string, verbose bool) bool {
	if len(files) == 0 {
		// If no specific files, unstage all
		if verbose {
			fmt.Println("Undoing git add with 'git restore --staged .'")
		}
		cmd := exec.Command("git", "restore", "--staged", ".")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run() == nil
	}

	if verbose {
		fmt.Printf("Undoing git add with 'git restore --staged %s'\n", strings.Join(files, " "))
	}

	args := append([]string{"restore", "--staged"}, files...)
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run() == nil
}

// undoBranch deletes a branch that was just created.
func undoBranch(branchName string, verbose bool) bool {
	if verbose {
		fmt.Printf("Undoing branch creation with 'git branch -D %s'\n", branchName)
	}

	cmd := exec.Command("git", "branch", "-D", branchName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run() == nil
}

// isReadOnlyGitCommand checks if a git command is read-only and shouldn't be logged
func isReadOnlyGitCommand(cmd string) bool {
	// Extract the git subcommand and arguments
	fields := strings.Fields(cmd)
	if len(fields) < 2 {
		return true // Invalid command format, treat as read-only
	}

	subCmd := fields[1]

	// Always read-only commands
	readOnlyCommands := map[string]bool{
		"status":      true,
		"log":         true,
		"blame":       true,
		"diff":        true,
		"show":        true,
		"ls-files":    true,
		"ls-remote":   true,
		"grep":        true,
		"shortlog":    true,
		"describe":    true,
		"rev-parse":   true,
		"cat-file":    true,
		"help":        true,
		"whatchanged": true,
		"reflog":      true,
		"name-rev":    true,
	}

	// If it's in the always read-only list
	if readOnlyCommands[subCmd] {
		return true
	}

	// Special cases that require argument inspection
	switch subCmd {
	case "remote":
		// "git remote" or "git remote -v" or "git remote show" are read-only
		// "git remote add", "git remote remove", etc. are not read-only
		if len(fields) == 2 || // just "git remote"
			(len(fields) == 3 && (fields[2] == "-v" || fields[2] == "show" || fields[2] == "get-url")) {
			return true
		}
		return false

	case "branch":
		// "git branch" with no args or with -l/-a/-r (listing) is read-only
		// "git branch <name>" (create) or "git branch -d/-D" (delete) are not read-only
		if len(fields) == 2 || // just "git branch"
			(len(fields) >= 3 && (fields[2] == "-l" || fields[2] == "-a" || fields[2] == "-r" ||
				fields[2] == "--list" || fields[2] == "--all" || fields[2] == "--remotes")) {
			return true
		}
		return false

	case "tag":
		// "git tag" with no args or with -l (listing) is read-only
		// "git tag <name>" (create) or "git tag -d" (delete) are not read-only
		if len(fields) == 2 || // just "git tag"
			(len(fields) >= 3 && (fields[2] == "-l" || fields[2] == "--list")) {
			return true
		}
		return false

	case "config":
		// "git config --get", "git config --list" are read-only
		// "git config <key> <value>" or "git config --global" sets are not read-only
		if len(fields) >= 3 && (fields[2] == "--get" || fields[2] == "--list" ||
			fields[2] == "-l" || fields[2] == "--get-all" ||
			fields[2] == "--get-regexp" || fields[2] == "--get-urlmatch") {
			return true
		}
		return false
	}

	// All other commands are considered modifying actions
	return false
}
