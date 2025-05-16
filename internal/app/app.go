package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/amberpixels/git-undo/internal/git-undo/config"
	"github.com/amberpixels/git-undo/internal/git-undo/logging"
	"github.com/amberpixels/git-undo/internal/git-undo/undoer"
	"github.com/amberpixels/git-undo/internal/githelpers"
	"github.com/mattn/go-shellwords"
)

// App represents the main application.
type App struct {
	verbose bool
	dryRun  bool
	repoDir string
}

// New creates a new App instance.
func New(verbose, dryRun bool) *App {
	return &App{
		verbose: verbose,
		dryRun:  dryRun,
	}
}

func (a *App) SetRepoDir(repoDir string) {
	a.repoDir = repoDir
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

	if a.repoDir != "" {
		// change current directory to a.repoDir
		if err := os.Chdir(a.repoDir); err != nil {
			return fmt.Errorf("failed to change directory to %s: %w", a.repoDir, err)
		}
	}

	// Ensure we're inside a Git repository
	if err := config.ValidateGitRepo(); err != nil {
		return err
	}

	// Custom commands are --hook and --log
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--hook"):
			return a.cmdHook(arg)
		case arg == "--log":
			return a.cmdLog()
		}
	}

	// Create a new logger
	logger, err := logging.NewLogger()
	if err != nil {
		return fmt.Errorf("logger initialization: %w", err)
	}

	// Check if this is a "git undo undo" command
	if len(args) > 0 && args[0] == "undo" {
		// Get the last undoed entry (from current reference)
		lastUndoedEntry, err := logger.GetEntry(logging.UndoedEntry)
		if err != nil {
			return fmt.Errorf("no command to redo: %w", err)
		}

		// Unmark the entry in the log
		marked, err := logger.ToggleEntry(lastUndoedEntry.Identifier)
		if err != nil {
			return fmt.Errorf("failed to unmark command: %w", err)
		}
		if marked {
			return fmt.Errorf("command was unexpectedly marked as undoed")
		}

		// Execute the original command
		words, err := shellwords.Parse(lastUndoedEntry.Command)
		if err != nil {
			return fmt.Errorf("invalid last undo-ed cmd: %w", err)
		}

		//nolint:gosec // TODO: future should we be safer here? Maybe let's sign our commands?
		cmd := exec.Command(words[0], words[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to redo command: %w", err)
		}

		a.logDebugf("Successfully redid: %s", lastUndoedEntry.Command)
		return nil
	}

	// Get the last git command
	lastEntry, err := logger.GetEntry(logging.RegularEntry)
	if err != nil {
		return fmt.Errorf("failed to get last git command: %w", err)
	}
	a.logDebugf("Looking for commands from current reference: [%s]", lastEntry.Ref)

	a.logDebugf("Last git command: %s", yellowColor+lastEntry.Command+resetColor)

	// Get the appropriate undoer
	u := undoer.New(lastEntry.Command)

	// Get the undo command
	undoCmd, err := u.GetUndoCommand()
	if err != nil {
		return err
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
		// Mark the entry as undoed in the log
		marked, err := logger.ToggleEntry(lastEntry.Identifier)
		if err != nil {
			a.logWarnf("Failed to mark command as undoed: %v", err)
		} else if !marked {
			a.logWarnf("Command was already marked as undoed")
		}
		a.logDebugf("Successfully undid: %s via %s", lastEntry.Command, undoCmd.Command)
		if len(undoCmd.Warnings) > 0 {
			for _, warning := range undoCmd.Warnings {
				a.logWarnf("%s", warning)
			}
		}
		return nil
	}

	return fmt.Errorf("failed to execute undo command %s via %s", lastEntry.Command, undoCmd.Command)
}

func (a *App) cmdHook(hookArg string) error {
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

// cmdLog displays the git-undo command log.
func (a *App) cmdLog() error {
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
