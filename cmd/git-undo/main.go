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

	if len(os.Args[1:]) == 1 && strings.HasPrefix(os.Args[1], "--hook") {
		val, ok := os.LookupEnv("GIT_UNDO_INTERNAL_HOOK")
		if !ok || val != "1" {
			if verbose {
				fmt.Fprintf(os.Stderr, "Unsuccessfull hook attempt")
			}
			// Hook MUST NOT be called by user, but by our zsh script
			os.Exit(1)
		}

		arg := os.Args[1]

		hooked := strings.TrimSpace(strings.TrimPrefix(arg, "--hook"))
		hooked = strings.TrimSpace(strings.TrimPrefix(hooked, "="))

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
