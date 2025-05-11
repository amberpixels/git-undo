package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitPaths holds important git repository paths
type GitPaths struct {
	RepoRoot string
	GitDir   string
	LogDir   string
}

// GetGitPaths retrieves relevant git repository paths
func GetGitPaths() (*GitPaths, error) {
	// Get the git repository root directory
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git repository root: %w", err)
	}
	repoRoot := strings.TrimSpace(string(output))

	// Get the git directory (usually .git, but could be elsewhere in worktrees)
	gitDirCmd := exec.Command("git", "rev-parse", "--git-dir")
	gitDirOutput, err := gitDirCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git directory: %w", err)
	}

	gitDir := strings.TrimSpace(string(gitDirOutput))

	// If gitDir is not an absolute path, make it absolute relative to the repo root
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(repoRoot, gitDir)
	}

	return &GitPaths{
		RepoRoot: repoRoot,
		GitDir:   gitDir,
		LogDir:   filepath.Join(gitDir, "undo-logs"),
	}, nil
}

// ValidateGitRepo checks if the current directory is inside a git repository
func ValidateGitRepo() error {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("not in a git repository")
	}
	return nil
}

// EnsureLogDir ensures the git-undo log directory exists
func EnsureLogDir(paths *GitPaths) error {
	if err := os.MkdirAll(paths.LogDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	return nil
}
