package githelpers

import (
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

// NewGitHelper creates a new GitHelper instance.
func NewGitHelper(repoDirArg ...string) *H {
	var repoDir string
	if len(repoDirArg) > 0 {
		repoDir = repoDirArg[0]
	}
	return &H{repoDir: repoDir}
}

// execGit executes a git command and returns its output as string.
func (g *H) execGit(subCmd string, args ...string) (string, error) {
	gitArgs := append([]string{subCmd}, args...)
	cmd := exec.Command("git", gitArgs...)
	cmd.Dir = g.repoDir

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// GetCurrentGitRef returns the current ref (branch, tag, commit hash) in the repository.
func (g *H) GetCurrentGitRef() (string, error) {
	// Try to get branch name first
	if ref, err := g.execGit("symbolic-ref", "--short", "HEAD"); err == nil {
		return ref, nil
	}

	// If not on a branch, try to get tag name
	if ref, err := g.execGit("describe", "--tags", "--exact-match"); err == nil {
		return ref, nil
	}

	// If not on a tag, get commit hash
	if ref, err := g.execGit("rev-parse", "--short", "HEAD"); err == nil {
		return ref, nil
	}

	return "", fmt.Errorf("failed to get current ref")
}

// GetRepoGitDir returns the path to the .git directory of current repository.
func (g *H) GetRepoGitDir() (string, error) {
	// Get the git directory (usually .git, but could be elsewhere in worktrees)
	gitDir, err := g.execGit("rev-parse", "--git-dir")
	if err != nil {
		return "", fmt.Errorf("failed to get git directory: %w", err)
	}

	// If gitDir is not an absolute path, make it absolute relative to the repo root
	if !filepath.IsAbs(gitDir) {
		repoRoot, err := g.execGit("rev-parse", "--show-toplevel")
		if err != nil {
			return "", fmt.Errorf("failed to get git repository root: %w", err)
		}

		gitDir = filepath.Join(repoRoot, gitDir)
	}

	return gitDir, nil
}
