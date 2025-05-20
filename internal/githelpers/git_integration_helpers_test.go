package githelpers_test

//import (
//	"os"
//	"path/filepath"
//	"testing"
//
//	"github.com/amberpixels/git-undo/internal/githelpers"
//	"github.com/amberpixels/git-undo/internal/testutil"
//	"github.com/stretchr/testify/suite"
//)
//
//type PathsTestSuite struct {
//	testutil.GitTestSuite
//}
//
//func TestPathsSuite(t *testing.T) {
//	suite.Run(t, new(PathsTestSuite))
//}
//
//func (s *PathsTestSuite) TestGetGitPaths() {
//	// Save current directory
//	originalDir, err := os.Getwd()
//	s.Require().NoError(err)
//	defer os.Chdir(originalDir)
//
//	// Change to test repository directory
//	err = os.Chdir(s.GetRepoDir())
//	s.Require().NoError(err)
//
//	paths, err := githelpers.GetGitPaths()
//	s.Require().NoError(err)
//	s.NotNil(paths)
//
//	// Get canonical path for test repo directory
//	canonicalRepoDir, err := filepath.EvalSymlinks(s.GetRepoDir())
//	s.Require().NoError(err)
//
//	// Verify paths are correct
//	s.Equal(canonicalRepoDir, paths.RepoRoot)
//	s.Equal(filepath.Join(canonicalRepoDir, ".git"), paths.RepoGitDir)
//}
//
//func (s *PathsTestSuite) TestGetGitPathsOutsideRepo() {
//	// Save current directory
//	originalDir, err := os.Getwd()
//	s.Require().NoError(err)
//	defer os.Chdir(originalDir)
//
//	// Change to a directory outside the git repo
//	tmpDir, err := os.MkdirTemp("", "gitundo-test-outside-*")
//	s.Require().NoError(err)
//	defer os.RemoveAll(tmpDir)
//
//	err = os.Chdir(tmpDir)
//	s.Require().NoError(err)
//
//	// Try to get paths - should fail
//	paths, err := githelpers.GetGitPaths()
//	s.Require().Error(err)
//	s.Nil(paths)
//	s.Contains(err.Error(), "failed to get git repository root")
//}
//
//func (s *PathsTestSuite) TestValidateGitRepo() {
//	// Should pass inside git repo
//	err := githelpers.ValidateGitRepo()
//	s.Require().NoError(err)
//
//	// Save current directory
//	originalDir, err := os.Getwd()
//	s.Require().NoError(err)
//	defer os.Chdir(originalDir)
//
//	// Change to a directory outside the git repo
//	tmpDir, err := os.MkdirTemp("", "gitundo-test-outside-*")
//	s.Require().NoError(err)
//	defer os.RemoveAll(tmpDir)
//
//	err = os.Chdir(tmpDir)
//	s.Require().NoError(err)
//
//	// Should fail outside git repo
//	err = githelpers.ValidateGitRepo()
//	s.Require().Error(err)
//	s.Equal("not in a git repository", err.Error())
//}
//
//func (s *PathsTestSuite) TestGetGitPathsWithWorktree() {
//	// Save current directory
//	originalDir, err := os.Getwd()
//	s.Require().NoError(err)
//	defer os.Chdir(originalDir)
//
//	// Get canonical path for test repo directory
//	canonicalRepoDir, err := filepath.EvalSymlinks(s.GetRepoDir())
//	s.Require().NoError(err)
//
//	// Create a worktree
//	worktreeDir := filepath.Join(s.GetRepoDir(), "worktree")
//	s.Git("worktree", "add", worktreeDir, "HEAD")
//
//	// Get canonical path for worktree directory
//	canonicalWorktreeDir, err := filepath.EvalSymlinks(worktreeDir)
//	s.Require().NoError(err)
//
//	// Change to worktree directory
//	err = os.Chdir(worktreeDir)
//	s.Require().NoError(err)
//
//	// Get paths from worktree
//	paths, err := githelpers.GetGitPaths()
//	s.Require().NoError(err)
//	s.NotNil(paths)
//
//	// Verify paths are correct
//	s.Equal(canonicalWorktreeDir, paths.RepoRoot)
//	s.Equal(filepath.Join(canonicalRepoDir, ".git", "worktrees", "worktree"), paths.RepoGitDir)
//}
