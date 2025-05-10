package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	// Parse command-line flags
	verbose := false
	fmt.Println("1")
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "--verbose" {
			verbose = true
		}

		fmt.Println(arg)

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

	// Find the last non-empty line that contains a git command
	for i := len(lines) - 1; i >= 0; i-- {
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
