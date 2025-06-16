package app

import (
	"errors"
	"fmt"
	"os"
	"strings"

	gitundoembeds "github.com/amberpixels/git-undo"
	"github.com/amberpixels/git-undo/internal/git-undo/logging"
	"github.com/amberpixels/git-undo/internal/git-undo/undoer"
	"github.com/amberpixels/git-undo/internal/githelpers"
)

// GitHelper provides methods for reading git references.
type GitHelper interface {
	GetCurrentGitRef() (string, error)
	GetRepoGitDir() (string, error)

	GitRun(subCmd string, args ...string) error
	GitOutput(subCmd string, args ...string) (string, error)
}

// App represents the main app.
type App struct {
	verbose      bool
	dryRun       bool
	buildVersion string

	dir string

	// isInternalCall is a hack, so app works OK even without GIT_UNDO_INTERNAL_HOOK env variable.
	// So, we can run tests without setting env vars (but just via setting this flag).
	// Note: here it's read-only flag, and it's only set in export_test.go
	isInternalCall bool

	// isBackMode indicates if this is git-back (true) or git-undo (false)
	isBackMode bool
}

// IsInternalCall checks if the hook is being called internally (either via test or zsh script).
func (a *App) IsInternalCall() bool {
	if a.isInternalCall {
		return true
	}

	val, ok := os.LookupEnv("GIT_UNDO_INTERNAL_HOOK")
	return ok && val == "1"
}

// NewAppGitUndo creates a new App instance.
func NewAppGitUndo(version string, verbose, dryRun bool) *App {
	return &App{
		dir:          ".",
		buildVersion: version,
		verbose:      verbose,
		dryRun:       dryRun,
		isBackMode:   false,
	}
}

// NewAppGiBack creates a new App instance for git-back.
func NewAppGiBack(version string, verbose, dryRun bool) *App {
	app := NewAppGitUndo(version, verbose, dryRun)
	app.isBackMode = true
	return app
}

// ANSI escape code for gray color.
const (
	yellowColor = "\033[33m"
	grayColor   = "\033[90m"
	redColor    = "\033[31m"
	resetColor  = "\033[0m"
)

// Application names.
const (
	appNameGitUndo = "git-undo"
	appNameGitBack = "git-back"
)

// getAppName returns the appropriate app name based on mode.
func (a *App) getAppName() string {
	if a.isBackMode {
		return appNameGitBack
	}
	return appNameGitUndo
}

// isCheckoutOrSwitchCommand checks if a command is a git checkout or git switch command.
func (a *App) isCheckoutOrSwitchCommand(command string) bool {
	// Parse the command to check its type
	gitCmd, err := githelpers.ParseGitCommand(command)
	if err != nil {
		return false
	}

	return gitCmd.Name == "checkout" || gitCmd.Name == "switch"
}

// logDebugf writes debug messages to stderr when verbose mode is enabled.
func (a *App) logDebugf(format string, args ...any) {
	if !a.verbose {
		return
	}

	_, _ = fmt.Fprintf(os.Stderr, yellowColor+a.getAppName()+" ⚙️: "+grayColor+format+resetColor+"\n", args...)
}

// logWarnf writes error messages to stderr.
func (a *App) logWarnf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, redColor+a.getAppName()+" ❌: "+grayColor+format+resetColor+"\n", args...)
}

// logInfof writes info messages to stderr.
func (a *App) logInfof(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, yellowColor+a.getAppName()+" ℹ️: "+grayColor+format+resetColor+"\n", args...)
}

// Run executes the main app logic.
func (a *App) Run(args []string) (err error) {
	a.logDebugf("called in verbose mode")

	defer func() {
		if recovered := recover(); recovered != nil {
			a.logDebugf("git-undo panic recovery: %v", recovered)
			err = errors.New("unexpected internal failure")
		}
	}()

	selfCtrl := NewSelfController(a.buildVersion, a.verbose, a.getAppName()).
		AddScript(CommandUpdate, gitundoembeds.GetUpdateScript()).
		AddScript(CommandUninstall, gitundoembeds.GetUninstallScript())

	if err := selfCtrl.HandleSelfCommand(args); err == nil {
		return nil
	} else if !errors.Is(err, ErrNotSelfCommand) {
		return err
	}

	g := githelpers.NewGitHelper(a.dir)

	gitDir, err := g.GetRepoGitDir()
	if err != nil {
		// Silently return for non-git repos when not using self commands
		a.logDebugf("not in a git repository, ignoring command%v: %s", args, err)
		return nil
	}

	lgr := logging.NewLogger(gitDir, g)
	if lgr == nil {
		return errors.New("failed to create git-undo logger")
	}

	// Custom commands are --hook and --log
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--hook"):
			return a.cmdHook(lgr, arg)
		case arg == "--log":
			return a.cmdLog(lgr)
		}
	}

	// Check if this is a "git undo undo" command
	if len(args) > 0 && args[0] == "undo" {
		// Get the last undoed entry (from current reference)
		lastEntry, err := lgr.GetLastEntry()
		if err != nil {
			a.logWarnf("something wrong with the log: %v", err)
			return nil
		}
		if lastEntry == nil || !lastEntry.Undoed {
			// nothing to undo
			return nil
		}

		// Unmark the entry in the log
		if err := lgr.ToggleEntry(lastEntry.GetIdentifier()); err != nil {
			return fmt.Errorf("failed to unmark command: %w", err)
		}

		// Execute the original command
		gitCmd, err := githelpers.ParseGitCommand(lastEntry.Command)
		if err != nil {
			return fmt.Errorf("invalid last undo-ed cmd[%s]: %w", lastEntry.Command, err)
		}
		if !gitCmd.Supported {
			return fmt.Errorf("invalid last undo-ed cmd[%s]: not supported", lastEntry.Command)
		}

		if err := g.GitRun(gitCmd.Name, gitCmd.Args...); err != nil {
			return fmt.Errorf("failed to redo command[%s]: %w", lastEntry.Command, err)
		}

		a.logDebugf("Successfully redid: %s", lastEntry.Command)
		return nil
	}

	// Get the last git command
	var lastEntry *logging.Entry
	if a.isBackMode {
		// For git-back, look for the last checkout/switch command (including undoed ones for toggle behavior)
		// We pass "any" to look across all refs, not just the current one
		lastEntry, err = lgr.GetLastEntry(logging.RefAny)
		if err != nil {
			return fmt.Errorf("failed to get last command: %w", err)
		}
		if lastEntry == nil {
			a.logDebugf("no commands found")
			return nil
		}
		// Check if the last command was a checkout or switch command
		if !a.isCheckoutOrSwitchCommand(lastEntry.Command) {
			// If not, try to find the last checkout/switch command (including undoed ones for toggle behavior)
			lastEntry, err = lgr.GetLastCheckoutSwitchEntryForToggle(logging.RefAny)
			if err != nil {
				return fmt.Errorf("failed to get last checkout/switch command: %w", err)
			}
			if lastEntry == nil {
				a.logDebugf("no checkout/switch commands to undo")
				return nil
			}
		}
	} else {
		// For git-undo, get any regular entry
		lastEntry, err = lgr.GetLastRegularEntry()
		if err != nil {
			return fmt.Errorf("failed to get last git command: %w", err)
		}
		if lastEntry == nil {
			a.logDebugf("nothing to undo")
			return nil
		}

		// Check if the last command was checkout or switch - suggest git back instead
		if a.isCheckoutOrSwitchCommand(lastEntry.Command) {
			a.logInfof("Last operation can't be undone. Use %sgit back%s instead.", yellowColor, resetColor)
			return nil
		}
	}

	a.logDebugf("Last git command[%s]: %s", lastEntry.Ref, yellowColor+lastEntry.Command+resetColor)

	// Get the appropriate undoer
	var u undoer.Undoer
	if a.isBackMode {
		u = undoer.NewBack(lastEntry.Command, g)
	} else {
		u = undoer.New(lastEntry.Command, g)
	}

	// Get the undo commands
	undoCmds, err := u.GetUndoCommands()
	if err != nil {
		return err
	}

	if a.dryRun {
		for _, undoCmd := range undoCmds {
			a.logDebugf("Would run: %s\n", undoCmd.Command)
			if len(undoCmd.Warnings) > 0 {
				for _, warning := range undoCmd.Warnings {
					a.logWarnf("%s", warning)
				}
			}
		}
		return nil
	}

	// Execute the undo commands
	for i, undoCmd := range undoCmds {
		if err := undoCmd.Exec(); err != nil {
			return fmt.Errorf("failed to execute undo command %d/%d %s via %s: %w",
				i+1, len(undoCmds), lastEntry.Command, undoCmd.Command, err)
		}
		a.logDebugf("Successfully executed undo command %d/%d: %s via %s",
			i+1, len(undoCmds), lastEntry.Command, undoCmd.Command)
		if len(undoCmd.Warnings) > 0 {
			for _, warning := range undoCmd.Warnings {
				a.logWarnf("%s", warning)
			}
		}
	}

	// Mark the entry as undoed in the log
	if err := lgr.ToggleEntry(lastEntry.GetIdentifier()); err != nil {
		a.logWarnf("Failed to mark command as undoed: %v", err)
	}

	// Summary message for all commands
	if len(undoCmds) == 1 {
		a.logDebugf("Successfully undid: %s via %s", lastEntry.Command, undoCmds[0].Command)
	} else {
		a.logDebugf("Successfully undid: %s via %d commands", lastEntry.Command, len(undoCmds))
	}
	return nil
}

func (a *App) cmdHook(lgr *logging.Logger, hookArg string) error {
	a.logDebugf("hook: start")

	if !a.IsInternalCall() {
		return errors.New("hook must be called from inside shell script (bash/zsh hook)")
	}

	hooked := strings.TrimSpace(strings.TrimPrefix(hookArg, "--hook"))
	hooked = strings.TrimSpace(strings.TrimPrefix(hooked, "="))

	gitCmd, err := githelpers.ParseGitCommand(hooked)
	if err != nil || !gitCmd.Supported {
		// This should not happen in a success path
		// because the zsh script should only send non-failed (so valid) git command
		// but just in case let's re-validate again here
		a.logDebugf("hook: skipping as invalid git command %q", hooked)
		return nil //nolint:nilerr // We're fine with this
	}
	if !gitCmd.ShouldBeLogged() {
		a.logDebugf("hook: skipping as a read-only command: %q", hooked)
		return nil
	}

	if err := lgr.LogCommand(hooked); err != nil {
		return fmt.Errorf("failed to log command: %w", err)
	}

	a.logDebugf("hook: prepended %q", hooked)
	return nil
}

// cmdLog displays the git-undo command log.
func (a *App) cmdLog(lgr *logging.Logger) error {
	return lgr.Dump(os.Stdout)
}
