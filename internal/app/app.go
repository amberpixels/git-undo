package app

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/amberpixels/git-undo/internal/git-undo/config"
	"github.com/amberpixels/git-undo/internal/git-undo/logging"
	"github.com/amberpixels/git-undo/internal/git-undo/undoer"
	gitHelpers "github.com/amberpixels/git-undo/internal/git_helpers"
)

// App represents the main application.
type App struct {
	verbose bool
	dryRun  bool
}

// New creates a new App instance.
func New(verbose, dryRun bool) *App {
	return &App{
		verbose: verbose,
		dryRun:  dryRun,
	}
}

// logDebugf writes debug messages to stderr when verbose mode is enabled.
func (a *App) logDebugf(format string, args ...interface{}) {
	if !a.verbose {
		return
	}

	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// logErrorf writes error messages to stderr.
func (a *App) logErrorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Run executes the main application logic.
func (a *App) Run(args []string) error {
	a.logDebugf("git-undo called in verbose mode")

	// Check if it was called in --hook mode
	// considered to be called only internally via shell hooks
	for _, arg := range args {
		if strings.HasPrefix(arg, "--hook") {
			return a.handleHookCommand(arg)
		}
	}

	if err := config.ValidateGitRepo(); err != nil {
		return fmt.Errorf("git repo validation: %w", err)
	}

	// Create a new logger
	logger, err := logging.NewLogger(a.verbose)
	if err != nil {
		return fmt.Errorf("logger initialization: %w", err)
	}

	// Get the last git command
	lastCmd, err := logger.GetLastCommand()
	if err != nil {
		return fmt.Errorf("failed to get last git command: %w", err)
	}

	a.logDebugf("Last git command: %s", lastCmd)

	// Get the appropriate undoer
	u, err := undoer.New(lastCmd)
	if err != nil {
		a.logErrorf("Cannot undo git command: %s. Supported commands: commit, add, branch", cmdDetails.SubCommand)
		return errors.New("unsupported command")
	}

	// Get the undo command
	undoCmd, err := u.GetUndoCommand()
	if err != nil {
		return fmt.Errorf("failed to undo: %w", err)
	}

	if a.dryRun {
		fmt.Printf("Would run: %s\n", undoCmd.Command)
		if len(undoCmd.Warnings) > 0 {
			for _, warning := range undoCmd.Warnings {
				fmt.Fprintf(os.Stderr, "%s\n", warning)
			}
		}
		return nil
	}

	// Execute the undo command
	if success := undoer.ExecuteUndoCommand(undoCmd); success {
		fmt.Printf("Successfully undid: %s via %s\n", lastCmd, undoCmd.Command)
		if len(undoCmd.Warnings) > 0 {
			for _, warning := range undoCmd.Warnings {
				fmt.Fprintf(os.Stderr, "%s\n", warning)
			}
		}
		return nil
	}

	return fmt.Errorf("failed to execute undo command %s via %s", lastCmd, undoCmd.Command)
}

func (a *App) handleHookCommand(hookArg string) error {
	a.logDebugf("hook: start")

	val, ok := os.LookupEnv("GIT_UNDO_INTERNAL_HOOK")
	if !ok || val != "1" {
		return errors.New("hook must be called by the zsh script")
	}

	hooked := strings.TrimSpace(strings.TrimPrefix(hookArg, "--hook"))
	hooked = strings.TrimSpace(strings.TrimPrefix(hooked, "="))

	if gitHelpers.IsReadOnlyGitCommand(hooked) {
		a.logDebugf("hook: skipping %q", hooked)
		return nil
	}

	logger, err := logging.NewLogger(a.verbose)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	if err := logger.LogCommand(hooked); err != nil {
		return fmt.Errorf("failed to log command: %w", err)
	}

	a.logDebugf("hook: prepended %q", hooked)
	return nil
}
