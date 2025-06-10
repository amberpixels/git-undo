package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	Self = "self"

	CommandUpdate    = "update"
	CommandUninstall = "uninstall"
	CommandVersion   = "version"
	CommandHelp      = "help"
)

// ErrNotSelfCommand is returned when the command is not a self command.
var ErrNotSelfCommand = errors.New("not a self command")

// allowedSelfCommands are the allowed self-management commands.
var allowedSelfCommands = []string{
	CommandUpdate,
	CommandUninstall,
	CommandVersion,
	CommandHelp,
}

// SelfController handles self-management commands that don't require a git repository.
type SelfController struct {
	buildVersion string
	verbose      bool
	appName      string

	// scripts is a map of self-management commands to their scripts.
	scripts map[string]string
}

// NewSelfController creates a new SelfController instance.
func NewSelfController(buildVersion string, verbose bool, appName string) *SelfController {
	return &SelfController{
		buildVersion: buildVersion,
		verbose:      verbose,
		appName:      appName,
		scripts:      map[string]string{},
	}
}

func (sc *SelfController) AddScript(cmdName, script string) *SelfController {
	sc.scripts[cmdName] = script
	return sc
}

// HandleSelfCommand processes self-management commands and returns true if handled.
// Returns (handled, error) where handled indicates if the command was a self command.
func (sc *SelfController) HandleSelfCommand(args []string) error {
	if len(args) == 0 {
		return ErrNotSelfCommand
	}

	selfCommand := sc.ExtractSelfCommand(args)
	if selfCommand == "" {
		return ErrNotSelfCommand
	}

	switch selfCommand {
	case CommandUpdate:
		if sc.appName == appNameGitBack {
			return fmt.Errorf("%s does not support update command. Use %s self update instead", appNameGitBack, appNameGitUndo)
		}
		return sc.cmdSelfUpdate()
	case CommandUninstall:
		if sc.appName == appNameGitBack {
			return fmt.Errorf("%s does not support uninstall command. Use %s self uninstall instead", appNameGitBack, appNameGitUndo)
		}
		return sc.cmdSelfUninstall()
	case CommandVersion:
		return sc.cmdVersion()
	case CommandHelp:
		return sc.cmdHelp()
	}

	return ErrNotSelfCommand
}

// ExtractSelfCommand checks if the given arguments represent a self-management command.
func (sc *SelfController) ExtractSelfCommand(args []string) string {
	if len(args) == 0 {
		return ""
	}

	var firstArg = args[0]
	var secondArg string
	if len(args) >= 2 {
		secondArg = args[1]
	}

	for _, cmd := range allowedSelfCommands {
		if firstArg == Self && secondArg == cmd || firstArg == fmt.Sprintf("%s-%s", Self, cmd) {
			return cmd
		}

		if secondArg == "" {
			cleanInput := strings.TrimPrefix(firstArg, "--")
			if cleanInput == CommandVersion || cleanInput == CommandHelp {
				return cleanInput
			}
		}
	}

	return ""
}

// cmdVersion displays the version information.
func (sc *SelfController) cmdVersion() error {
	fmt.Fprintf(os.Stdout, "%s\n", sc.buildVersion)
	return nil
}

// cmdHelp displays the help information.
func (sc *SelfController) cmdHelp() error {
	if sc.appName == appNameGitBack {
		fmt.Fprintf(os.Stdout, "%s %s\n", appNameGitBack, sc.buildVersion)
		fmt.Fprintf(os.Stdout, "Usage: %s\n", appNameGitBack)
		fmt.Fprintf(os.Stdout, "\n")
		fmt.Fprintf(os.Stdout, "Git-back undoes the last git checkout or git switch command,\n")
		fmt.Fprintf(os.Stdout, "returning you to the previous branch or commit.\n")
		fmt.Fprintf(os.Stdout, "\n")
		fmt.Fprintf(os.Stdout, "Commands:\n")
		fmt.Fprintf(os.Stdout, "  version   Display %s version\n", appNameGitBack)
		fmt.Fprintf(os.Stdout, "  help      Display this help\n")
		return nil
	}

	// Default git-undo help
	fmt.Fprintf(os.Stdout, "%s %s\n", appNameGitUndo, sc.buildVersion)
	fmt.Fprintf(os.Stdout, "Usage: %s [command]\n", appNameGitUndo)
	fmt.Fprintf(os.Stdout, "\n")
	fmt.Fprintf(os.Stdout, "Commands:\n")
	fmt.Fprintf(os.Stdout, "  update    Update %s to the latest version\n", appNameGitUndo)
	fmt.Fprintf(os.Stdout, "  uninstall Uninstall %s\n", appNameGitUndo)
	fmt.Fprintf(os.Stdout, "  version   Display %s version\n", appNameGitUndo)
	fmt.Fprintf(os.Stdout, "  help      Display this help\n")
	return nil
}

// cmdSelfUpdate runs the embedded self-update script.
func (sc *SelfController) cmdSelfUpdate() error {
	sc.logDebugf("Running embedded self-update script...")
	updateScript, ok := sc.scripts[CommandUpdate]
	if !ok {
		return errors.New("update script not available")
	}

	return sc.runEmbeddedScript(updateScript, "update")
}

// cmdSelfUninstall runs the embedded self-uninstall script.
func (sc *SelfController) cmdSelfUninstall() error {
	sc.logDebugf("Running embedded self-uninstall script...")
	uninstallScript, ok := sc.scripts[CommandUninstall]
	if !ok {
		return errors.New("uninstall script not available")
	}

	return sc.runEmbeddedScript(uninstallScript, "uninstall")
}

// runEmbeddedScript creates a temporary script file and executes it.
func (sc *SelfController) runEmbeddedScript(script, name string) error {
	if script == "" {
		return fmt.Errorf("embedded %s script not available", name)
	}

	// Create temp file with proper extension
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("%s-%s-*.sh", appNameGitUndo, name))
	if err != nil {
		return fmt.Errorf("failed to create temp script: %w", err)
	}
	defer func() {
		// TODO: handle error: log warnings at least
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()

	// Write script content
	if _, err := tmpFile.WriteString(script); err != nil {
		return fmt.Errorf("failed to write script: %w", err)
	}

	// Close file before making it executable and running it
	_ = tmpFile.Close()

	// Make executable
	//nolint:gosec // TODO: fix me in future
	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		return fmt.Errorf("failed to make script executable: %w", err)
	}

	sc.logDebugf("Executing embedded %s script...", name)

	// Execute script
	//nolint:gosec // TODO: fix me in future
	cmd := exec.Command("bash", tmpFile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// logDebugf writes debug messages to stderr when verbose mode is enabled.
func (sc *SelfController) logDebugf(format string, args ...interface{}) {
	if !sc.verbose {
		return
	}

	fmt.Fprintf(os.Stderr, yellowColor+sc.appName+" ⚙️: "+grayColor+format+resetColor+"\n", args...)
}
