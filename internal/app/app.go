package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/amberpixels/git-undo/internal/git-undo/config"
	"github.com/amberpixels/git-undo/internal/git-undo/logging"
	"github.com/amberpixels/git-undo/internal/git-undo/undoer"
	"github.com/amberpixels/git-undo/internal/githelpers"
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

// ANSI escape code for gray color.
const (
	yellowColor = "\033[33m"
	grayColor   = "\033[90m"
	redColor    = "\033[31m"
	resetColor  = "\033[0m"
)

// logDebugf writes debug messages to stderr when verbose mode is enabled.
func (a *App) logDebugf(format string, args ...interface{}) {
	if !a.verbose {
		return
	}

	fmt.Fprintf(os.Stderr, yellowColor+"git-undo ⚙️: "+grayColor+format+resetColor+"\n", args...)
}

// logWarnf writes error messages to stderr.
func (a *App) logWarnf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, redColor+"git-undo ❌: "+grayColor+format+resetColor+"\n", args...)
}

// Run executes the main application logic.
func (a *App) Run(args []string) error {
	a.logDebugf("called in verbose mode")

	// Check if it was called in --hook mode
	// considered to be called only internally via shell hooks
	for _, arg := range args {
		if strings.HasPrefix(arg, "--hook") {
			return a.handleHookCommand(arg)
		}
		if arg == "--log" {
			return a.handleLogCommand()
		}
	}

	if err := config.ValidateGitRepo(); err != nil {
		return fmt.Errorf("git repo validation: %w", err)
	}

	// Create a new logger
	logger, err := logging.NewLogger()
	if err != nil {
		return fmt.Errorf("logger initialization: %w", err)
	}

	// Get the last git command
	lastCmd, err := logger.GetLastCommand()
	if err != nil {
		return fmt.Errorf("failed to get last git command: %w", err)
	}

	a.logDebugf("Last git command: %s", yellowColor+lastCmd+resetColor)

	// Get the appropriate undoer
	u, err := undoer.New(lastCmd)
	if err != nil {
		return fmt.Errorf(
			"unsupported command: %s. Supported commands: %s",
			redColor+lastCmd+grayColor,
			yellowColor+"add, commit, branch"+grayColor,
		)
	}

	// Get the undo command
	undoCmd, err := u.GetUndoCommand()
	if err != nil {
		return fmt.Errorf("failed to undo: %w", err)
	}

	if a.dryRun {
		a.logDebugf("Would run: %s\n", undoCmd.Command)
		if len(undoCmd.Warnings) > 0 {
			for _, warning := range undoCmd.Warnings {
				a.logWarnf("%s", warning)
			}
		}
		return nil
	}

	// Execute the undo command
	if success := undoer.ExecuteUndoCommand(undoCmd); success {
		// Mark the command as undoed in the log
		if err := logger.MarkCommandAsUndoed(); err != nil {
			a.logWarnf("Failed to mark command as undoed: %v", err)
		}
		a.logDebugf("Successfully undid: %s via %s", lastCmd, undoCmd.Command)
		if len(undoCmd.Warnings) > 0 {
			for _, warning := range undoCmd.Warnings {
				a.logWarnf("%s", warning)
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

	gitCmd := githelpers.ParseGitCommand(hooked)
	if !gitCmd.Valid {
		// This should not happen in a success path
		// because the zsh script should only send non-failed (so valid) git command
		// but just in case let's re-validate again here
		a.logDebugf("hook: skipping as invalid git command %q", hooked)
		return nil
	}
	if gitCmd.IsReadOnly {
		a.logDebugf("hook: skipping as a read-only command: %q", hooked)
		return nil
	}

	logger, err := logging.NewLogger()
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	if err := logger.LogCommand(hooked); err != nil {
		return fmt.Errorf("failed to log command: %w", err)
	}

	a.logDebugf("hook: prepended %q", hooked)
	return nil
}

// handleLogCommand displays the git-undo command log.
func (a *App) handleLogCommand() error {
	logger, err := logging.NewLogger()
	if err != nil {
		return fmt.Errorf("logger initialization: %w", err)
	}

	// Get the log file path
	logFile := filepath.Join(logger.GetLogDir(), "command-log.txt")

	// Check if log file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		return errors.New("no command log found. Run some git commands first")
	}

	// Read and display the log file
	content, err := os.ReadFile(logFile)
	if err != nil {
		return fmt.Errorf("failed to read log file: %w", err)
	}

	// Print the log content
	fmt.Fprintf(os.Stdout, "%s", string(content))
	return nil
}
