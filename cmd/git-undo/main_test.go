package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

// GitTestSuite provides a test environment for git operations.
type GitTestSuite struct {
	suite.Suite
	repoDir string
}

// SetupSuite creates a temporary directory and initializes a git repository.
func (s *GitTestSuite) SetupSuite() {
	tmp, err := os.MkdirTemp("", "gitundo-test-*")
	s.Require().NoError(err)
	s.repoDir = tmp

	// Initialize git repository
	s.git("init", ".")
	s.git("commit", "--allow-empty", "-m", "init")
}

// TearDownSuite cleans up the temporary directory.
func (s *GitTestSuite) TearDownSuite() {
	if s.repoDir != "" {
		os.RemoveAll(s.repoDir)
	}
}

// git runs a git command in the test repository and automatically simulates the hook.
func (s *GitTestSuite) git(args ...string) {
	// Run the actual git command
	output := s.runCmd("git", args...)
	_ = output

	// Simulate the hook by constructing the command string
	cmdStr := "git " + strings.Join(args, " ")
	_ = s.gitUndoWithHook(cmdStr)
}

// gitUndo runs git-undo with the given arguments.
func (s *GitTestSuite) gitUndo(args ...string) {
	_ = s.runCmd("git-undo", args...)
}

// gitUndoWithHook runs git-undo with the hook environment variable set.
func (s *GitTestSuite) gitUndoWithHook(hookCmd string) string {
	return s.runCmdWithEnv([]string{"GIT_UNDO_INTERNAL_HOOK=1"}, "git-undo", "--hook="+hookCmd)
}

// runCmd executes a command in the test repository.
func (s *GitTestSuite) runCmd(cmd string, args ...string) string {
	return s.runCmdWithEnv(nil, cmd, args...)
}

// runCmdWithEnv executes a command with additional environment variables.
func (s *GitTestSuite) runCmdWithEnv(extraEnv []string, cmd string, args ...string) string {
	c := exec.Command(cmd, args...)
	c.Dir = s.repoDir
	if extraEnv != nil {
		c.Env = append(os.Environ(), extraEnv...)
	}
	out, err := c.CombinedOutput()
	s.Require().NoError(err, "Command failed: %s %v\n%s", cmd, args, out)
	return string(out)
}

// assertBranchExists checks if a branch exists.
func (s *GitTestSuite) assertBranchExists(branch string) {
	output := s.runCmd("git", "branch", "--list", branch)
	s.NotEmpty(output, "Expected branch %s to exist", branch)
}

// assertBranchNotExists checks if a branch doesn't exist.
func (s *GitTestSuite) assertBranchNotExists(branch string) {
	output := s.runCmd("git", "branch", "--list", branch)
	s.Empty(output, "Expected branch %s to not exist", branch)
}

// TestGitUndoSuite runs all git-undo related tests.
func TestGitUndoSuite(t *testing.T) {
	suite.Run(t, new(GitTestSuite))
}

// TestUndoBranch tests the branch deletion functionality.
func (s *GitTestSuite) TestUndoBranch() {
	// Create a branch - hook is automatically simulated
	s.git("branch", "feature")
	s.assertBranchExists("feature")

	// Run undo
	s.gitUndo()

	// Verify branch is gone
	s.assertBranchNotExists("feature")
}

// TestUndoAdd tests the git add undo functionality.
func (s *GitTestSuite) TestUndoAdd() {
	// Create a test file
	testFile := filepath.Join(s.repoDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	s.Require().NoError(err)

	// Add the file - hook is automatically simulated
	s.git("add", "test.txt")

	// Verify file is staged
	status := s.runCmd("git", "status", "--porcelain")
	s.Contains(status, "A  test.txt", "File should be staged")

	// Run undo
	s.gitUndo()

	// Verify file is unstaged
	status = s.runCmd("git", "status", "--porcelain")
	s.Contains(status, "?? test.txt", "File should be unstaged")
}
