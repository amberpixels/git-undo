package app_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amberpixels/git-undo/internal/app"
	"github.com/amberpixels/git-undo/internal/testutil"
	"github.com/stretchr/testify/suite"
)

const (
	verbose         = false
	autoGitUndoHook = true
)

// GitTestSuite provides a test environment for git operations.
type GitTestSuite struct {
	testutil.GitTestSuite
	application *app.App
}

// TestGitUndoSuite runs all git-undo related tests.
func TestGitUndoSuite(t *testing.T) {
	suite.Run(t, new(GitTestSuite))
}

func (s *GitTestSuite) SetupTest() {
	// throw away any changes, drop untracked files
	root := strings.TrimSpace(s.RunCmd("git", "rev-list", "--max-parents=0", "HEAD"))
	s.RunCmd("git", "reset", "--hard", root)
	// Remove any untracked files (just in case)
	s.RunCmd("git", "clean", "-fdx")
}

// SetupSuite initializes the test suite and creates the application instance.
func (s *GitTestSuite) SetupSuite() {
	s.GitTestSuite.Verbose = verbose
	s.GitTestSuite.GitUndoHook = autoGitUndoHook
	s.GitTestSuite.SetupSuite()

	s.application = app.New(verbose, false)
	s.application.SetRepoDir(s.GetRepoDir())
}

// gitUndo runs git-undo with the given arguments.
func (s *GitTestSuite) gitUndo(args ...string) {
	err := s.application.Run(args)
	s.Require().NoError(err)
}

func (s *GitTestSuite) gitUndoLog() string {
	// Capture stdout
	r, w, err := os.Pipe()
	s.Require().NoError(err)
	origStdout := os.Stdout
	//nolint:reassign // TODO: fix this in future
	os.Stdout = w

	// Run the log command
	err = s.application.Run([]string{"--log"})
	// Close the writer end and restore stdout
	_ = w.Close()
	//nolint:reassign // TODO: fix this in future
	os.Stdout = origStdout
	s.Require().NoError(err)

	// Read captured output
	outBytes, err := io.ReadAll(r)
	s.Require().NoError(err)
	output := string(outBytes)

	// Split into lines and limit to 10
	lines := strings.Split(output, "\n")
	if len(lines) > 10 {
		lines = lines[:10]
	}
	return strings.Join(lines, "\n")
}

// TestUndoBranch tests the branch deletion functionality.
func (s *GitTestSuite) TestUndoBranch() {
	// Create a branch - hook is automatically simulated
	s.Git("branch", "feature")
	s.AssertBranchExists("feature")

	// Run undo
	s.gitUndo()

	// Verify branch is gone
	s.AssertBranchNotExists("feature")
}

// TestUndoAdd tests the git add undo functionality.
func (s *GitTestSuite) TestUndoAdd() {
	// Create a test file
	testFile := filepath.Join(s.GetRepoDir(), "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	s.Require().NoError(err)

	// Add the file - hook is automatically simulated
	s.Git("add", "test.txt")

	// Verify file is staged
	status := s.RunCmd("git", "status", "--porcelain")
	s.Contains(status, "A  test.txt", "File should be staged")

	// Run undo
	s.gitUndo()

	// Verify file is unstaged
	status = s.RunCmd("git", "status", "--porcelain")
	s.Contains(status, "?? test.txt", "File should be unstaged")
}

// TestSequentialUndo tests multiple undo operations in sequence.
func (s *GitTestSuite) TestSequentialUndo() {
	// Create test files
	file1 := filepath.Join(s.GetRepoDir(), "file1.txt")
	file2 := filepath.Join(s.GetRepoDir(), "file2.txt")
	err := os.WriteFile(file1, []byte("content1"), 0644)
	s.Require().NoError(err)
	err = os.WriteFile(file2, []byte("content2"), 0644)
	s.Require().NoError(err)

	// First sequence: add and commit file1
	s.Git("add", "file1.txt")
	s.Git("commit", "-m", "First commit")

	// Second sequence: add and commit file2
	s.Git("add", "file2.txt")
	s.Git("commit", "-m", "Second commit")

	// Verify both files are committed
	status := s.RunCmd("git", "status", "--porcelain")
	s.Empty(status, "No files should be modified or staged")

	// First undo: should undo the second commit (keeping file2.txt staged)
	s.gitUndo()
	status = s.RunCmd("git", "status", "--porcelain")
	s.Contains(status, "A  file2.txt", "file2.txt should be staged after undoing second commit")

	// Second undo: should unstage file2.txt
	s.gitUndo()
	status = s.RunCmd("git", "status", "--porcelain")
	s.Contains(status, "?? file2.txt", "file2.txt should be untracked after undoing add")

	// Third undo: should undo the first commit (keeping file1.txt staged)
	s.gitUndo()
	status = s.RunCmd("git", "status", "--porcelain")
	s.Contains(status, "A  file1.txt", "file1.txt should be staged after undoing first commit")

	// Fourth undo: should unstage file1.txt
	s.gitUndo()
	status = s.RunCmd("git", "status", "--porcelain")
	s.Contains(status, "?? file1.txt", "file1.txt should be untracked after undoing add")
}

// TestUndoLog tests that the git-undo log command works and shows output.
func (s *GitTestSuite) TestUndoLog() {
	// Create and commit a file
	testFile := filepath.Join(s.GetRepoDir(), "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	s.Require().NoError(err)

	// Perform some git operations to generate log entries
	s.Git("add", "test.txt")
	s.Git("commit", "-m", "'First commit'")

	// Check that log command works and shows output
	log := s.gitUndoLog()
	s.NotEmpty(log, "Log should not be empty")
	s.Contains(log, "git commit -m 'First commit'")
	s.Contains(log, "git add test.txt")
}

// TestUndoUndo tests the git undo undo (redo) functionality.
func (s *GitTestSuite) TestUndoUndo() {
	// Create a test file
	testFile := filepath.Join(s.GetRepoDir(), "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	s.Require().NoError(err)

	// Add and commit the file
	s.Git("add", "test.txt")
	s.Git("commit", "-m", "'First commit'")

	status := s.RunCmd("git", "status", "--porcelain")
	s.Empty(status, "status is empty as everything is commited")

	// Undo the commit
	s.gitUndo()
	status = s.RunCmd("git", "status", "--porcelain")
	s.Contains(status, "A  test.txt", "File should be staged after undoing commit")

	// Undo the undo of the commit
	s.gitUndo("undo")
	status = s.RunCmd("git", "status", "--porcelain")
	s.Empty(status, "status is empty as everything is commited back (undo undo)")
}

// TestUndoStash tests the git stash undo functionality.
func (s *GitTestSuite) TestUndoStash() {
	// Create a test file
	testFile := filepath.Join(s.GetRepoDir(), "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	s.Require().NoError(err)

	// Add and commit initial state
	s.Git("add", "test.txt")
	s.Git("commit", "-m", "Initial commit")

	// Modify the file
	err = os.WriteFile(testFile, []byte("modified content"), 0644)
	s.Require().NoError(err)

	// Verify file is modified
	status := s.RunCmd("git", "status", "--porcelain")
	s.Contains(status, " M test.txt", "File should be modified")

	// Stash the changes
	s.Git("stash")

	// Verify working directory is clean
	status = s.RunCmd("git", "status", "--porcelain")
	s.Empty(status, "Working directory should be clean after stash")

	// Run undo
	s.gitUndo()

	// Verify changes are restored
	status = s.RunCmd("git", "status", "--porcelain")
	s.Contains(status, " M test.txt", "File should be modified after undo")

	// Verify stash list is empty (stash was dropped)
	stashList := s.RunCmd("git", "stash", "list")
	s.Empty(stashList, "Stash list should be empty after undo")
}
