package app_test

import (
	"context"
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
	verbose              = false
	autoGitUndoHook      = true
	testAppVersion       = "v1.2.3-test"
	testAppVersionSource = "fixed"
)

// GitTestSuite provides a test environment for git operations.
type GitTestSuite struct {
	testutil.GitTestSuite
	app *app.App
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

// SetupSuite initializes the test suite and creates the app instance.
func (s *GitTestSuite) SetupSuite() {
	s.GitTestSuite.Verbose = verbose
	s.GitTestSuite.GitUndoHook = autoGitUndoHook
	s.GitTestSuite.SetupSuite()

	s.app = app.NewAppGitUndo(testAppVersion, testAppVersionSource)
	app.SetupAppDir(s.app, s.GetRepoDir())
	app.SetupInternalCall(s.app)
	s.GitTestSuite.SetApplication(s.app)
}

// gitUndo runs git-undo with the given arguments.
func (s *GitTestSuite) gitUndo(args ...string) {
	opts := app.RunOptions{
		Args: args,
	}
	err := s.app.Run(context.Background(), opts)
	s.Require().NoError(err)
}

func (s *GitTestSuite) gitUndoLog() string {
	// Capture stdout
	r, w, err := os.Pipe()
	s.Require().NoError(err)
	origStdout := os.Stdout
	setGlobalStdout(w)

	// Run the log command
	opts := app.RunOptions{
		ShowLog: true,
	}
	err = s.app.Run(context.Background(), opts)
	// Close the writer end and restore stdout
	_ = w.Close()
	setGlobalStdout(origStdout)
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
	s.Git("add", filepath.Base(testFile))

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
	// Setup: Create an initial base commit so we're not working from the root commit
	initialFile := filepath.Join(s.GetRepoDir(), "initial.txt")
	err := os.WriteFile(initialFile, []byte("initial content"), 0644)
	s.Require().NoError(err)
	s.Git("add", filepath.Base(initialFile))
	s.Git("commit", "-m", "Initial base commit")

	// Create test files
	file1 := filepath.Join(s.GetRepoDir(), "file1.txt")
	file2 := filepath.Join(s.GetRepoDir(), "file2.txt")
	err = os.WriteFile(file1, []byte("content1"), 0644)
	s.Require().NoError(err)
	err = os.WriteFile(file2, []byte("content2"), 0644)
	s.Require().NoError(err)

	// First sequence: add and commit file1
	s.Git("add", filepath.Base(file1))
	s.Git("commit", "-m", "First commit")

	// Second sequence: add and commit file2
	s.Git("add", filepath.Base(file2))
	s.Git("commit", "-m", "Second commit")

	// Verify both files are committed
	status := s.RunCmd("git", "status", "--porcelain")
	s.Empty(status, "No files should be modified or staged")

	// First undo: should undo the second commit (keeping file2.txt staged)
	s.gitUndo()
	status = s.RunCmd("git", "status", "--porcelain")
	s.Contains(status, "A  file2.txt", "file2.txt should be staged after undoing second commit")
	s.NotContains(status, "A  file1.txt", "file1.txt should not be staged after undoing second commit")

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

	// Create and switch to a new branch
	s.Git("checkout", "-b", "feature-branch")

	// Perform some git operations to generate log entries
	s.Git("add", filepath.Base(testFile)) // Use relative path for git commands
	s.Git("commit", "-m", "First commit")

	// Check that log command works and shows output
	log := s.gitUndoLog()
	s.NotEmpty(log, "Log should not be empty")
	s.Contains(log, "git commit -m First commit", "Log should contain commit command")
	s.Contains(log, "git add test.txt", "Log should contain add command")
	s.Contains(log, "|feature-branch|", "Log should contain branch name")

	// Switch back to main and verify the branch name changes in the log
	s.Git("checkout", "main")
	log = s.gitUndoLog()
	s.Contains(log, "|main|", "Log should contain updated branch name")
}

// TestUndoUndo tests the git undo undo (redo) functionality.
func (s *GitTestSuite) TestUndoUndo() {
	// Create a test file
	testFile := filepath.Join(s.GetRepoDir(), "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	s.Require().NoError(err)

	// Add and commit the file
	s.Git("add", filepath.Base(testFile))
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
	s.Git("add", filepath.Base(testFile))
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

// TestCheckoutSwitchDetection tests that git undo warns about checkout/switch commands.
//
//nolint:reassign // in tests it's OK
func (s *GitTestSuite) TestCheckoutSwitchDetection() {
	// Create a test branch
	s.Git("branch", "test-branch")

	// Perform a checkout operation
	s.Git("checkout", "test-branch")

	// Capture stderr to check warning output
	r, w, err := os.Pipe()
	s.Require().NoError(err)
	origStderr := os.Stderr
	os.Stderr = w

	// Run git undo - should warn about checkout
	opts := app.RunOptions{}
	err = s.app.Run(context.Background(), opts)

	// Close writer and restore stderr
	_ = w.Close()
	os.Stderr = origStderr

	// Should not error
	s.Require().NoError(err)

	// Read captured output
	outBytes, err := io.ReadAll(r)
	s.Require().NoError(err)
	output := string(outBytes)

	// Should contain warning about checkout and suggest git back
	s.Contains(output, "can't be undone", "Should warn that it can't be undone")
	s.Contains(output, "git back", "Should suggest using git back")

	// Now test with git switch
	s.Git("switch", "main")
	s.Git("switch", "test-branch")

	// Capture stderr again
	r, w, err = os.Pipe()
	s.Require().NoError(err)
	os.Stderr = w

	// Run git undo - should warn about switch
	opts = app.RunOptions{}
	err = s.app.Run(context.Background(), opts)

	// Close writer and restore stderr
	_ = w.Close()
	os.Stderr = origStderr

	// Should not error
	s.Require().NoError(err)

	// Read captured output
	outBytes, err = io.ReadAll(r)
	s.Require().NoError(err)
	output = string(outBytes)

	// Should contain warning about switch and suggest git back
	s.Contains(output, "can't be undone", "Should warn that it can't be undone")
	s.Contains(output, "git back", "Should suggest using git back")
}

// TestUndoMerge tests the git merge undo functionality.
func (s *GitTestSuite) TestUndoMerge() {
	// Create and switch to a new branch
	s.Git("checkout", "-b", "feature")

	// Create a test file in feature branch
	testFile := filepath.Join(s.GetRepoDir(), "feature.txt")
	err := os.WriteFile(testFile, []byte("feature content"), 0644)
	s.Require().NoError(err)

	// Add and commit the file in feature branch
	s.Git("add", filepath.Base(testFile))
	s.Git("commit", "-m", "Feature commit")

	// Switch back to main and create a different commit
	s.Git("checkout", "main")
	mainFile := filepath.Join(s.GetRepoDir(), "main.txt")
	err = os.WriteFile(mainFile, []byte("main content"), 0644)
	s.Require().NoError(err)
	s.Git("add", filepath.Base(mainFile))
	s.Git("commit", "-m", "Main commit")

	// Merge feature into main
	s.Git("merge", "feature")

	// Verify both files exist
	_, err = os.Stat(testFile)
	s.Require().NoError(err, "Feature file should exist after merge")
	_, err = os.Stat(mainFile)
	s.Require().NoError(err, "Main file should exist after merge")

	// Run undo
	s.gitUndo()

	// Verify feature file no longer exists in working directory
	_, err = os.Stat(testFile)
	s.Require().Error(err, "Feature file should not exist after undoing merge")
	_, err = os.Stat(mainFile)
	s.Require().NoError(err, "Main file should still exist after undoing merge")
}

// TestSelfCommands tests the self-management commands.
func (s *GitTestSuite) TestSelfCommands() {
	s.T().Skip("Skipping self commands test") // TODO: fix me in future

	// These commands should work even outside a git repo
	// We'll just test that they don't error out and attempt to call the scripts

	// Create a temporary app without git repo requirement for this test
	testApp := app.NewAppGitUndo(testAppVersion, testAppVersionSource)

	// Test self update command - these will actually try to run the real scripts
	// but should fail on network/permission issues rather than script issues
	opts := app.RunOptions{Args: []string{"self", "update"}}
	err := testApp.Run(context.Background(), opts)
	s.Require().Error(err) // Expected to fail in test environment

	// Test self-update command (hyphenated form)
	opts = app.RunOptions{Args: []string{"self-update"}}
	err = testApp.Run(context.Background(), opts)
	s.Require().Error(err) // Expected to fail in test environment

	// Test self uninstall command
	opts = app.RunOptions{Args: []string{"self", "uninstall"}}
	err = testApp.Run(context.Background(), opts)
	s.Require().Error(err) // Expected to fail in test environment

	// Test self-uninstall command (hyphenated form)
	opts = app.RunOptions{Args: []string{"self-uninstall"}}
	err = testApp.Run(context.Background(), opts)
	s.Require().Error(err) // Expected to fail in test environment
}

// TestSelfCommandsParsing tests that self commands are parsed correctly without requiring git repo.
func (s *GitTestSuite) TestSelfCommandsParsing() {
	// Test that self commands bypass git repo validation
	s.T().Skip("Skipping self commands parsing test") // TODO: fix me in future

	// Create a temporary directory that's NOT a git repo
	tmpDir := s.T().TempDir()
	_ = tmpDir
	// Create an app pointing to the non-git directory
	testApp := app.NewAppGitUndo(testAppVersion, testAppVersionSource)
	s.Require().NotNil(testApp)

	// These should attempt to run (and fail on script execution) rather than fail on git repo validation
	testCases := [][]string{
		{"self", "update"},
		{"self-update"},
		{"self", "uninstall"},
		{"self-uninstall"},
	}

	for _, args := range testCases {
		opts := app.RunOptions{Args: args}
		err := testApp.Run(context.Background(), opts)
		// Should fail on script execution, not on git repo validation
		s.Require().Error(err, "Command %v should fail on script execution", args)
		// Should not contain git repo error
		s.NotContains(err.Error(), "not a git repository", "Command %v should not fail on git repo validation", args)
	}
}

// TestVersionCommands tests all the different ways to call the version command.
func (s *GitTestSuite) TestVersionCommands() {
	// Test all version command variations
	testCases := [][]string{
		{"version"},
		{"--version"},
		{"self-version"},
		{"self", "version"},
	}

	for _, args := range testCases {
		// Capture stdout to check version output
		r, w, err := os.Pipe()
		s.Require().NoError(err)
		origStdout := os.Stdout
		setGlobalStdout(w)

		// Run the version command
		opts := app.RunOptions{Args: args}
		err = s.app.Run(context.Background(), opts)

		// Close writer and restore stdout
		_ = w.Close()
		setGlobalStdout(origStdout)

		// Should not error
		s.Require().NoError(err, "Version command %v should not error", args)

		// Read captured output
		outBytes, err := io.ReadAll(r)
		s.Require().NoError(err)
		output := string(outBytes)

		// Should contain version
		s.Contains(output, "v1.2.3-test", "Version command %v should output version", args)
	}
}

// TestVersionDetection tests the version detection priority.
func (s *GitTestSuite) TestVersionDetection() {
	s.T().Skip("Skipping version detection test") // TODO: fix me in future

	// Test with git version available (in actual git repo)
	gitApp := app.NewAppGitUndo(testAppVersion, testAppVersionSource)

	s.Require().NotNil(gitApp)

	// Capture stdout to check git version
	r, w, err := os.Pipe()
	s.Require().NoError(err)
	origStdout := os.Stdout
	setGlobalStdout(w)

	opts := app.RunOptions{Args: []string{"version"}}
	err = gitApp.Run(context.Background(), opts)

	_ = w.Close()
	setGlobalStdout(origStdout)
	s.Require().NoError(err)

	outBytes, err := io.ReadAll(r)
	s.Require().NoError(err)
	gitOutput := string(outBytes)

	// Should show git version (not "unknown" or build version)
	s.Contains(gitOutput, "git-undo", "Should contain git-undo")
	s.NotContains(gitOutput, "unknown", "Should not show unknown when git is available")

	// Test with build version only (no git repo)
	tmpDir := s.T().TempDir()
	buildApp := app.NewAppGitUndo(testAppVersion, testAppVersionSource)

	_ = tmpDir

	r, w, err = os.Pipe()
	s.Require().NoError(err)
	setGlobalStdout(w)

	opts = app.RunOptions{Args: []string{"version"}}
	err = buildApp.Run(context.Background(), opts)

	_ = w.Close()
	setGlobalStdout(origStdout)
	s.Require().NoError(err)

	outBytes, err = io.ReadAll(r)
	s.Require().NoError(err)
	buildOutput := string(outBytes)

	// Should show build version
	s.Contains(buildOutput, "git-undo v2.0.0-build", "Should show build version when no git")

	// Test fallback to unknown
	unknownApp := app.NewAppGitUndo(testAppVersion, testAppVersionSource)

	// Don't set build version

	r, w, err = os.Pipe()
	s.Require().NoError(err)
	setGlobalStdout(w)

	opts = app.RunOptions{Args: []string{"version"}}
	err = unknownApp.Run(context.Background(), opts)

	_ = w.Close()
	setGlobalStdout(origStdout)
	s.Require().NoError(err)

	outBytes, err = io.ReadAll(r)
	s.Require().NoError(err)
	unknownOutput := string(outBytes)

	// Should show unknown
	s.Contains(unknownOutput, "git-undo unknown", "Should show unknown when no version available")
}

func setGlobalStdout(f *os.File) {
	os.Stdout = f //nolint:reassign // we're fine with this for now
}
