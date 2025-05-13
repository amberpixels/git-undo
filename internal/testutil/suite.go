package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/stretchr/testify/suite"
)

// GitTestSuite provides a test environment for git operations.
type GitTestSuite struct {
	suite.Suite
	repoDir     string
	Verbose     bool
	GitUndoHook bool
}

// SetupSuite creates a temporary directory and initializes a git repository.
func (s *GitTestSuite) SetupSuite() {
	tmp, err := os.MkdirTemp("", "gitundo-test-*")
	s.Require().NoError(err)
	s.repoDir = tmp

	// Initialize git repository
	s.Git("init", ".")
	s.Git("commit", "--allow-empty", "-m", "init")
}

// TearDownSuite cleans up the temporary directory.
func (s *GitTestSuite) TearDownSuite() {
	if s.repoDir == "" {
		return
	}

	s.Require().NoError(os.RemoveAll(s.repoDir))
}

// Git runs a git command in the test repository.
func (s *GitTestSuite) Git(args ...string) {
	_ = s.RunCmd("git", args...)

	if s.GitUndoHook {
		_ = s.RunCmdWithEnv([]string{"GIT_UNDO_INTERNAL_HOOK=1"}, "git-undo", "--hook="+"git "+strings.Join(args, " "))
	}
}

// RunCmd executes a command in the test repository.
func (s *GitTestSuite) RunCmd(cmd string, args ...string) string {
	return s.RunCmdWithEnv(nil, cmd, args...)
}

// RunCmdWithEnv executes a command with additional environment variables.
func (s *GitTestSuite) RunCmdWithEnv(extraEnv []string, cmd string, args ...string) string {
	if s.Verbose {
		var envStr string
		if extraEnv != nil {
			envStr = fmt.Sprintf("ENV=`%s`", strings.Join(extraEnv, " "))
		}
		//nolint:forbidigo // it's ok for TESTS
		fmt.Printf("# %s %s %s\n", cmd, strings.Join(args, " "), envStr)
	}

	c := exec.Command(cmd, args...)
	c.Dir = s.repoDir
	if extraEnv != nil {
		c.Env = append(os.Environ(), extraEnv...)
	}
	out, err := c.CombinedOutput()
	s.Require().NoError(err, "Command failed: %s %v\n%s", cmd, args, out)

	result := string(out)
	if s.Verbose && result != "" {
		//nolint:forbidigo // it's ok for TESTS
		fmt.Printf("> %s\n", result)
	}

	return result
}

// GetRepoDir returns the repository directory path.
func (s *GitTestSuite) GetRepoDir() string {
	return s.repoDir
}

// CreateFile creates a file in the repository with the given content.
func (s *GitTestSuite) CreateFile(name, content string) {
	filePath := filepath.Join(s.repoDir, name)
	err := os.WriteFile(filePath, []byte(content), 0600)
	s.Require().NoError(err)
}

// AssertFileExists checks if a file exists in the repository.
func (s *GitTestSuite) AssertFileExists(name string) {
	filePath := filepath.Join(s.repoDir, name)
	_, err := os.Stat(filePath)
	s.NoError(err, "File %s should exist", name)
}

// AssertFileNotExists checks if a file doesn't exist in the repository.
func (s *GitTestSuite) AssertFileNotExists(name string) {
	filePath := filepath.Join(s.repoDir, name)
	_, err := os.Stat(filePath)
	s.True(os.IsNotExist(err), "File %s should not exist", name)
}

// AssertBranchExists checks if a branch exists.
func (s *GitTestSuite) AssertBranchExists(branch string) {
	output := s.RunCmd("git", "branch", "--list", branch)
	s.NotEmpty(output, "Expected branch %s to exist", branch)
}

// AssertBranchNotExists checks if a branch doesn't exist.
func (s *GitTestSuite) AssertBranchNotExists(branch string) {
	output := s.RunCmd("git", "branch", "--list", branch)
	s.Empty(output, "Expected branch %s to not exist", branch)
}
