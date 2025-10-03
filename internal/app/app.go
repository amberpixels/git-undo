package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"runtime/debug"

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
	version       string
	versionSource string

	// dir stands for working dir for the App
	// It's suggested to be filled and used in tests only.
	dir string

	// isInternalCall is a hack, so app works OK even without GIT_UNDO_INTERNAL_HOOK env variable.
	// So, we can run tests without setting env vars (but just via setting this flag).
	// Note: here it's read-only flag, and it's only set in export_test.go
	isInternalCall bool

	// isBackMode indicates if this is git-back (true) or git-undo (false)
	isBackMode bool
}

// getIsInternalCall checks if the hook is being called internally (either via test or zsh script).
func (a *App) getIsInternalCall() bool {
	if a.isInternalCall {
		return true
	}

	val, ok := os.LookupEnv("GIT_UNDO_INTERNAL_HOOK")
	return ok && val == "1"
}

// NewAppGitUndo creates a new App instance.
func NewAppGitUndo(version string, versionSource string) *App {
	return &App{
		dir:           ".",
		version:       version,
		versionSource: versionSource,
		isBackMode:    false,
	}
}

// NewAppGitBack creates a new App instance for git-back.
func NewAppGitBack(version, versionSource string) *App {
	app := NewAppGitUndo(version, versionSource)
	app.isBackMode = true
	return app
}

// HandleVersion handles the --version flag by delegating to SelfController.
func (a *App) HandleVersion(ctx context.Context, verbose bool) error {
	selfCtrl := NewSelfController(ctx, a.version, a.versionSource, verbose, a.getAppName())
	return selfCtrl.cmdVersion()
}

// RunOptions contains parsed CLI options.
type RunOptions struct {
	Verbose     bool
	DryRun      bool
	HookCommand string
	ShowLog     bool
	Args        []string
}

// Run executes the app with parsed options.
func (a *App) Run(ctx context.Context, opts RunOptions) error {
	a.logDebugf(opts.Verbose, "called in verbose mode")

	defer func() {
		if recovered := recover(); recovered != nil {
			a.logDebugf(opts.Verbose, "git-undo panic recovery: %v", recovered)
		}
	}()

	selfCtrl := NewSelfController(ctx, a.version, a.versionSource, opts.Verbose, a.getAppName()).
		AddScript(CommandUpdate, gitundoembeds.GetUpdateScript()).
		AddScript(CommandUninstall, gitundoembeds.GetUninstallScript())

	if err := selfCtrl.HandleSelfCommand(opts.Args); err == nil {
		return nil
	} else if !errors.Is(err, ErrNotSelfCommand) {
		return err
	}

	g := githelpers.NewGitHelper(ctx, a.dir)

	gitDir, err := g.GetRepoGitDir()
	if err != nil {
		// Silently return for non-git repos when not using self commands
		a.logDebugf(opts.Verbose, "not in a git repository, ignoring command%v: %s", opts.Args, err)
		return nil
	}

	lgr := logging.NewLogger(gitDir, g)
	if lgr == nil {
		return errors.New("failed to create git-undo logger")
	}

	// Handle --hook flag
	if opts.HookCommand != "" {
		return a.cmdHook(lgr, opts.Verbose, opts.HookCommand)
	}

	// Handle --log flag
	if opts.ShowLog {
		return a.cmdLog(lgr)
	}

	return a.run(ctx, lgr, g, opts)
}

// run contains the core undo/back functionality.
func (a *App) run(ctx context.Context, lgr *logging.Logger, g GitHelper, opts RunOptions) error {
	if a.isBackMode {
		return a.runBack(ctx, lgr, g, opts)
	}

	// Determine the operation type based on args and app mode
	// `git undo undo` -> redo
	if len(opts.Args) > 0 && opts.Args[0] == githelpers.CustomCommandUndo {
		return a.runRedo(ctx, lgr, g, opts)
	}

	// This is git-undo
	return a.runUndo(ctx, lgr, g, opts)
}

// runRedo handles "git undo undo" operations (redo functionality).
func (a *App) runRedo(_ context.Context, lgr *logging.Logger, g GitHelper, opts RunOptions) error {
	a.logDebugf(opts.Verbose, "runRedo called")

	// Get the last undoed entry (from current reference)
	lastEntry, err := lgr.GetLastUndoedEntry()
	if err != nil {
		a.logErrorf("something wrong with the log: %v", err)
		return nil
	}
	if lastEntry == nil {
		// nothing to redo
		a.logInfof("nothing to redo")
		return nil
	}

	a.logDebugf(opts.Verbose, "runRedo: found undoed entry: %s", lastEntry.Command)

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

	a.logDebugf(opts.Verbose, "Successfully redid: %s", lastEntry.Command)
	return nil
}

// runBack handles git-back operations (navigation undo).
func (a *App) runBack(ctx context.Context, lgr *logging.Logger, g GitHelper, opts RunOptions) error {
	// For git-back, look for the last checkout/switch command (including undoed ones for toggle behavior)
	// We pass "any" to look across all refs, not just the current one
	lastEntry, err := lgr.GetLastEntry(logging.RefAny)
	if err != nil {
		return fmt.Errorf("failed to get last command: %w", err)
	}
	if lastEntry == nil {
		a.logInfof("no commands found")
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
			a.logInfof("no checkout/switch commands to undo")
			return nil
		}
	}

	return a.executeUndoOperation(ctx, lgr, g, opts, lastEntry, true)
}

// runUndo handles git-undo operations (mutation undo).
func (a *App) runUndo(ctx context.Context, lgr *logging.Logger, g GitHelper, opts RunOptions) error {
	// First, check if the chronologically last command was a checkout/switch command
	absoluteLastEntry, err := lgr.GetLastEntry()
	if err != nil {
		return fmt.Errorf("failed to get last command: %w", err)
	}

	if absoluteLastEntry != nil && a.isCheckoutOrSwitchCommand(absoluteLastEntry.Command) {
		a.logInfof("Last operation can't be undone. Use %sgit back%s instead.", yellowColor, resetColor)
		return nil
	}

	// For git-undo, get the last regular (mutation) entry to undo
	lastEntry, err := lgr.GetLastRegularEntry()
	if err != nil {
		return fmt.Errorf("failed to get last git command: %w", err)
	}

	if lastEntry == nil {
		// Check if there are any navigation commands if no mutation commands exist
		lastNavEntry, err := lgr.GetLastCheckoutSwitchEntry()
		if err != nil {
			return fmt.Errorf("failed to get last checkout/switch command: %w", err)
		}
		if lastNavEntry != nil {
			a.logInfof("Last operation can't be undone. Use %sgit back%s instead.", yellowColor, resetColor)
			return nil
		}
		a.logInfof("nothing to undo")
		return nil
	}

	// Check if the last regular command was checkout or switch - suggest git back instead
	if a.isCheckoutOrSwitchCommand(lastEntry.Command) {
		a.logInfof("Last operation can't be undone. Use %sgit back%s instead.", yellowColor, resetColor)
		return nil
	}

	return a.executeUndoOperation(ctx, lgr, g, opts, lastEntry, false)
}

// executeUndoOperation performs the actual undo operation for a given entry.
func (a *App) executeUndoOperation(
	ctx context.Context,
	lgr *logging.Logger,
	g GitHelper,
	opts RunOptions,
	lastEntry *logging.Entry,
	isBackMode bool,
) error {
	a.logDebugf(opts.Verbose, "Last git command[%s]: %s", lastEntry.Ref, yellowColor+lastEntry.Command+resetColor)

	// Get the appropriate undoer
	var u undoer.Undoer
	if isBackMode {
		u = undoer.NewBack(lastEntry.Command, g)
	} else {
		u = undoer.New(lastEntry.Command, g)
	}

	// Get the undo commands
	undoCmds, err := u.GetUndoCommands()
	if err != nil {
		return err
	}

	if opts.DryRun {
		return a.showDryRunOutput(opts, undoCmds)
	}

	// Execute the undo commands
	if err := a.executeUndoCommands(ctx, opts, lastEntry, undoCmds); err != nil {
		return err
	}

	// Mark the entry as undoed in the log
	if err := lgr.ToggleEntry(lastEntry.GetIdentifier()); err != nil {
		a.logWarnf("Failed to mark command as undoed: %v", err)
	}

	// Summary message
	a.logUndoSummary(opts, lastEntry, undoCmds)
	return nil
}

// showDryRunOutput displays what would be executed in dry-run mode.
func (a *App) showDryRunOutput(opts RunOptions, undoCmds []*undoer.UndoCommand) error {
	for _, undoCmd := range undoCmds {
		a.logDebugf(opts.Verbose, "Would run: %s\n", undoCmd.Command)
		if len(undoCmd.Warnings) > 0 {
			for _, warning := range undoCmd.Warnings {
				a.logWarnf("%s", warning)
			}
		}
	}
	return nil
}

// executeUndoCommands executes the list of undo commands.
func (a *App) executeUndoCommands(
	ctx context.Context,
	opts RunOptions,
	lastEntry *logging.Entry,
	undoCmds []*undoer.UndoCommand,
) error {
	for i, undoCmd := range undoCmds {
		// TODO: at some point we can check ctx here for timeout/cancel/etc
		_ = ctx

		if err := undoCmd.Exec(); err != nil {
			return fmt.Errorf("failed to execute undo command %d/%d %s via %s: %w",
				i+1, len(undoCmds), lastEntry.Command, undoCmd.Command, err)
		}
		a.logDebugf(opts.Verbose, "Successfully executed undo command %d/%d: %s via %s",
			i+1, len(undoCmds), lastEntry.Command, undoCmd.Command)
		if len(undoCmd.Warnings) > 0 {
			for _, warning := range undoCmd.Warnings {
				a.logWarnf("%s", warning)
			}
		}
	}
	return nil
}

// logUndoSummary logs a summary message after successful undo operation.
func (a *App) logUndoSummary(opts RunOptions, lastEntry *logging.Entry, undoCmds []*undoer.UndoCommand) {
	if len(undoCmds) == 1 {
		a.logDebugf(opts.Verbose, "Successfully undid: %s via %s", lastEntry.Command, undoCmds[0].Command)
	} else {
		a.logDebugf(opts.Verbose, "Successfully undid: %s via %d commands", lastEntry.Command, len(undoCmds))
	}
}

// ANSI escape code for gray color.
const (
	yellowColor = "\033[33m"
	orangeColor = "\033[38;5;208m"
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
func (a *App) logDebugf(verbose bool, format string, args ...any) {
	if !verbose {
		return
	}

	_, _ = fmt.Fprintf(os.Stderr, yellowColor+a.getAppName()+" ⚙️: "+grayColor+format+resetColor+"\n", args...)
}

// logErrorf writes error messages to stderr.
func (a *App) logErrorf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, redColor+a.getAppName()+" ❌️: "+grayColor+format+resetColor+"\n", args...)
}

// logWarnf writes warning (soft error) messages to stderr.
func (a *App) logWarnf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, orangeColor+a.getAppName()+" ⚠️: "+grayColor+format+resetColor+"\n", args...)
}

// logInfof writes info messages to stderr.
func (a *App) logInfof(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, yellowColor+a.getAppName()+" ℹ️: "+grayColor+format+resetColor+"\n", args...)
}

func (a *App) cmdHook(lgr *logging.Logger, verbose bool, hooked string) error {
	a.logDebugf(verbose, "hook: start")

	if !a.getIsInternalCall() {
		return errors.New("hook must be called from inside shell script (bash/zsh hook)")
	}

	hooked = strings.TrimSpace(hooked)

	gitCmd, err := githelpers.ParseGitCommand(hooked)
	if err != nil || !gitCmd.Supported {
		// This should not happen in a success path
		// because the zsh script should only send non-failed (so valid) git command
		// but just in case let's re-validate again here
		a.logDebugf(verbose, "hook: skipping as invalid git command %q", hooked)
		return nil //nolint:nilerr // We're fine with this
	}
	if !logging.ShouldBeLogged(gitCmd) {
		a.logDebugf(verbose, "hook: skipping as a read-only command: %q", hooked)
		return nil
	}

	if err := lgr.LogCommand(hooked); err != nil {
		return fmt.Errorf("failed to log command: %w", err)
	}

	a.logDebugf(verbose, "hook: prepended %q", hooked)
	return nil
}

// cmdLog displays the git-undo command log.
func (a *App) cmdLog(lgr *logging.Logger) error {
	return lgr.Dump(os.Stdout)
}

// HandleError prints error messages and exits with status code 1.
func HandleError(appName string, err error) {
	_, _ = fmt.Fprintln(os.Stderr, redColor+appName+" ❌: "+grayColor+err.Error()+resetColor)
	os.Exit(1)
}

// HandleAppVersion handles the app binary version.
func HandleAppVersion(ldFlagVersion, versionSource string) (string, string) {
	// 1. `build way`: by default version is given via `go build` from ldflags
	var version = ldFlagVersion
	if version != "" {
		versionSource = "ldflags"
	}

	// 2. `install way`: When running binary that was installed via `go install`, here we'll get the proper version
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		version = bi.Main.Version
		versionSource = "buildinfo"
	}

	return version, versionSource
}
