package undoer

import (
	"fmt"
	"strings"
)

// SwitchUndoer handles undoing git switch operations.
type SwitchUndoer struct {
	git GitExec

	originalCmd *CommandDetails
}

var _ Undoer = &SwitchUndoer{}

// GetUndoCommands returns the commands that would undo the switch operation.
func (s *SwitchUndoer) GetUndoCommands() ([]*UndoCommand, error) {
	// Handle switch -c as branch creation (similar to checkout -b)
	for i, arg := range s.originalCmd.Args {
		if (arg == "-c" || arg == "--create") && i+1 < len(s.originalCmd.Args) {
			branchName := s.originalCmd.Args[i+1]
			return []*UndoCommand{NewUndoCommand(s.git,
				fmt.Sprintf("git branch -D %s", branchName),
				fmt.Sprintf("Delete branch '%s' created by switch -c", branchName),
			)}, nil
		}
		// Handle switch -C as force branch creation (overwrites existing branch)
		if (arg == "-C" || arg == "--force-create") && i+1 < len(s.originalCmd.Args) {
			branchName := s.originalCmd.Args[i+1]
			// For force create, we can't easily restore the previous branch state
			// so we provide a warning and delete the branch
			return []*UndoCommand{NewUndoCommand(s.git,
				fmt.Sprintf("git branch -D %s", branchName),
				fmt.Sprintf("Delete branch '%s' created by switch -C", branchName),
				"Warning: switch -C may have overwritten an existing branch that cannot be restored",
			)}, nil
		}
	}

	// Handle regular branch switching (go back to previous branch)
	// This is similar to git checkout behavior - we want to return to previous branch

	// First, check if we have a previous branch to go back to
	prevBranch, err := s.git.GitOutput("rev-parse", "--symbolic-full-name", "@{-1}")
	if err != nil {
		return nil, fmt.Errorf("%w: no previous branch to return to", ErrUndoNotSupported)
	}

	prevBranch = strings.TrimSpace(prevBranch)
	if prevBranch == "" {
		return nil, fmt.Errorf("%w: no previous branch to return to", ErrUndoNotSupported)
	}

	// Remove the refs/heads/ prefix if present to get just the branch name
	prevBranch = strings.TrimPrefix(prevBranch, "refs/heads/")

	// Check working directory status to detect potential conflicts
	var warnings []string

	// Check for staged changes
	stagedOutput, err := s.git.GitOutput("diff", "--cached", "--name-only")
	if err == nil && strings.TrimSpace(stagedOutput) != "" {
		warnings = append(warnings, "You have staged changes that may conflict with branch switching")
	}

	// Check for unstaged changes
	unstagedOutput, err := s.git.GitOutput("diff", "--name-only")
	if err == nil && strings.TrimSpace(unstagedOutput) != "" {
		warnings = append(warnings, "You have unstaged changes that may conflict with branch switching")
	}

	// Check for untracked files
	untrackedOutput, err := s.git.GitOutput("ls-files", "--others", "--exclude-standard")
	if err == nil && strings.TrimSpace(untrackedOutput) != "" {
		warnings = append(warnings, "You have untracked files (these usually don't conflict)")
	}

	// Add helpful suggestions if there are potential conflicts
	if len(warnings) > 0 {
		warnings = append(warnings, "If switch undo fails, try: 'git stash' first, then undo, then 'git stash pop'")
		warnings = append(warnings, "Or commit your changes first with 'git commit -m \"WIP\"'")
	}

	// Use "git switch -" to go back to the previous branch
	// git switch supports the same "-" syntax as git checkout
	return []*UndoCommand{NewUndoCommand(s.git,
		"git switch -",
		fmt.Sprintf("Switch back to previous branch (%s)", prevBranch),
		warnings...,
	)}, nil
}
