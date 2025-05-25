package githelpers

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// H is the struct for wrapping git helper functions
// It holds repoDir where commands are executed.
type H struct {
	repoDir string
}

const invalidRepoDir = "<invalid repo dir>"

// NewGitHelper creates a new GitHelper instance.
func NewGitHelper(repoDirArg ...string) *H {
	h := &H{}

	if len(repoDirArg) > 0 {
		h.repoDir = repoDirArg[0]
	} else {
		// get repoDir of the current directory
		var err error
		if h.repoDir, err = h.execGitOutput("rev-parse", "--show-toplevel"); err != nil {
			h.repoDir = invalidRepoDir
		}
	}
	return h
}

// execGitOutput executes a git command and returns its output as string.
func (h *H) execGitOutput(subCmd string, args ...string) (string, error) {
	if h.repoDir == invalidRepoDir {
		return "", errors.New("not a valid git repository")
	}

	gitArgs := append([]string{subCmd}, args...)
	cmd := exec.Command("git", gitArgs...)
	cmd.Dir = h.repoDir

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// execGitRun executes a git command without output (via Run).
func (h *H) execGitRun(subCmd string, args ...string) error {
	if h.repoDir == invalidRepoDir {
		return errors.New("not a valid git repository")
	}

	gitArgs := append([]string{subCmd}, args...)
	cmd := exec.Command("git", gitArgs...)
	cmd.Dir = h.repoDir

	return cmd.Run()
}

// validateGitRepo checks if the current directory is inside a git repository.
func (h *H) validateGitRepo() (string, error) {
	gitDir, err := h.execGitOutput("rev-parse", "--git-dir")
	if err != nil {
		return "", errors.New("not in a git repository")
	}

	return gitDir, nil
}

// GetCurrentGitRef returns the current ref (branch, tag, commit hash) in the repository.
func (h *H) GetCurrentGitRef() (string, error) {
	// Try to get branch name first
	if ref, err := h.execGitOutput("symbolic-ref", "--short", "HEAD"); err == nil {
		return ref, nil
	}

	// If not on a branch, try to get tag name
	if ref, err := h.execGitOutput("describe", "--tags", "--exact-match"); err == nil {
		return ref, nil
	}

	// If not on a tag, get commit hash
	if ref, err := h.execGitOutput("rev-parse", "--short", "HEAD"); err == nil {
		return ref, nil
	}

	return "", errors.New("failed to get current ref")
}

// GetRepoGitDir returns the path to the .git directory of current repository.
func (h *H) GetRepoGitDir() (string, error) {
	// Get the git directory (usually .git, but could be elsewhere in worktrees)
	gitDir, err := h.validateGitRepo()
	if err != nil {
		return "", err
	}

	// If gitDir is not an absolute path, make it absolute relative to the repo root
	if !filepath.IsAbs(gitDir) {
		repoRoot, err := h.execGitOutput("rev-parse", "--show-toplevel")
		if err != nil {
			return "", fmt.Errorf("failed to get git repository root: %w", err)
		}

		gitDir = filepath.Join(repoRoot, gitDir)
	}

	return gitDir, nil
}

// GitRun executes a git command without output (via Run).
func (h *H) GitRun(subCmd string, args ...string) error {
	return h.execGitRun(subCmd, args...)
}

// GitOutput executes a git command and returns its output as string.
func (h *H) GitOutput(subCmd string, args ...string) (string, error) {
	return h.execGitOutput(subCmd, args...)
}
