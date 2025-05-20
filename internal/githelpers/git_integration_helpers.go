package githelpers

//// GitPaths holds important git repository paths.
//type GitPaths struct {
//	RepoRoot   string
//	RepoGitDir string
//}
//
//// GetGitPaths retrieves relevant git repository paths.
//func GetGitPaths() (*GitPaths, error) {
//	// Get the git repository root directory
//	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
//	output, err := cmd.Output()
//	if err != nil {
//		return nil, fmt.Errorf("failed to get git repository root: %w", err)
//	}
//	repoRoot := strings.TrimSpace(string(output))
//
//	// Get the git directory (usually .git, but could be elsewhere in worktrees)
//	gitDirCmd := exec.Command("git", "rev-parse", "--git-dir")
//	gitDirOutput, err := gitDirCmd.Output()
//	if err != nil {
//		return nil, fmt.Errorf("failed to get git directory: %w", err)
//	}
//
//	gitDir := strings.TrimSpace(string(gitDirOutput))
//
//	// If gitDir is not an absolute path, make it absolute relative to the repo root
//	if !filepath.IsAbs(gitDir) {
//		gitDir = filepath.Join(repoRoot, gitDir)
//	}
//
//	return &GitPaths{
//		RepoRoot:   repoRoot,
//		RepoGitDir: gitDir,
//	}, nil
//}
//
//// ValidateGitRepo checks if the current directory is inside a git repository.
//func ValidateGitRepo() error {
//	cmd := exec.Command("git", "rev-parse", "--git-dir")
//	if err := cmd.Run(); err != nil {
//		return errors.New("not in a git repository")
//	}
//	return nil
//}
