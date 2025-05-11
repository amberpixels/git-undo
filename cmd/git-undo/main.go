package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/amberpixels/git-undo/internal/git-undo/command"
	"github.com/amberpixels/git-undo/internal/git-undo/config"
	"github.com/amberpixels/git-undo/internal/git-undo/logging"
)

func isVerbose() bool {
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "--verbose" {
			return true
		}
	}
	return false
}

func isDryRun() bool {
	for _, arg := range os.Args[1:] {
		if arg == "--dry-run" {
			return true
		}
	}
	return false
}

func main() {
	// Parse command-line flags
	verbose := isVerbose()
	dryRun := isDryRun()
	if verbose {
		fmt.Fprintf(os.Stderr, "git-undo process called\n")
	}

	args := os.Args[1:]

	if idx, hookArg := isMatchingArg(args, func(arg string) bool {
		return strings.HasPrefix(arg, "--hook")
	}); idx >= 0 {
		if err := handleHookCommand(hookArg, verbose); err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Hook failed: %s\n", err)
			}
			os.Exit(1)
		}
		os.Exit(0)
	}

	if err := config.ValidateGitRepo(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Create a new logger
	logger, err := logging.NewLogger(verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Get the last git command
	lastCmd, err := logger.GetLastCommand()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to get last git command: %v\n", err)
		os.Exit(1)
	}

	if verbose {
		fmt.Printf("Last git command: %s\n", lastCmd)
	}

	// Parse the command
	cmdDetails, err := command.ParseCommand(lastCmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Get the appropriate undoer
	undoer, err := command.GetUndoer(cmdDetails)
	if err != nil {
		fmt.Printf("Cannot undo git command: %s\n", cmdDetails.SubCommand)
		fmt.Println("Supported commands: commit, add, branch")
		os.Exit(1)
	}

	// Get the undo command
	undoCmd, err := undoer.GetUndoCommand(verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if dryRun {
		fmt.Printf("Would run: %s\n", undoCmd)
		os.Exit(0)
	}

	// Execute the undo command
	if success := command.ExecuteUndoCommand(undoCmd); success {
		fmt.Printf("Successfully undid: %s\n", lastCmd)
	} else {
		fmt.Fprintf(os.Stderr, "Failed to undo: %s\n", lastCmd)
		os.Exit(1)
	}
}

func isMatchingArg(args []string, cb func(arg string) bool) (int, string) {
	for idx, arg := range args {
		if cb(arg) {
			return idx, arg
		}
	}
	return -1, ""
}

func handleHookCommand(hookArg string, verbose bool) error {
	if verbose {
		fmt.Fprintf(os.Stderr, "hook: start\n")
	}

	val, ok := os.LookupEnv("GIT_UNDO_INTERNAL_HOOK")
	if !ok || val != "1" {
		return fmt.Errorf("hook must be called by the zsh script")
	}

	hooked := strings.TrimSpace(strings.TrimPrefix(hookArg, "--hook"))
	hooked = strings.TrimSpace(strings.TrimPrefix(hooked, "="))

	if command.IsReadOnlyCommand(hooked) {
		if verbose {
			fmt.Fprintf(os.Stderr, "hook: skipping %q\n", hooked)
		}
		return nil
	}

	logger, err := logging.NewLogger(verbose)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	if err := logger.LogCommand(hooked); err != nil {
		return fmt.Errorf("failed to log command: %w", err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "hook: prepended %q\n", hooked)
	}
	return nil
}
