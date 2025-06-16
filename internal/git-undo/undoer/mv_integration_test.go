package undoer_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/amberpixels/git-undo/internal/git-undo/undoer"
	"github.com/amberpixels/git-undo/internal/githelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMvUndoer_Integration_MultipleFiles tests the actual execution of multi-file mv undo
// This replicates the bug found in BATS test "1A: Phase 1 Commands".
func TestMvUndoer_Integration_MultipleFiles(t *testing.T) {
	// Create a temporary directory for our git repo
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	err := cmd.Run()
	require.NoError(t, err)

	// Configure git for testing
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	err = cmd.Run()
	require.NoError(t, err)

	// Create initial commit
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "init")
	cmd.Dir = tmpDir
	err = cmd.Run()
	require.NoError(t, err)

	// Create test files
	file1Path := filepath.Join(tmpDir, "file1.txt")
	file2Path := filepath.Join(tmpDir, "file2.txt")
	subdirPath := filepath.Join(tmpDir, "subdir")

	err = os.WriteFile(file1Path, []byte("file1 content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2Path, []byte("file2 content"), 0644)
	require.NoError(t, err)

	// Stage and commit the files
	cmd = exec.Command("git", "add", "file1.txt", "file2.txt")
	cmd.Dir = tmpDir
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "commit", "-m", "Add files for move test")
	cmd.Dir = tmpDir
	err = cmd.Run()
	require.NoError(t, err)

	// Create subdirectory
	err = os.MkdirAll(subdirPath, 0755)
	require.NoError(t, err)

	// Move multiple files to subdirectory (this is the operation we want to undo)
	cmd = exec.Command("git", "mv", "file1.txt", "file2.txt", "subdir/")
	cmd.Dir = tmpDir
	err = cmd.Run()
	require.NoError(t, err)

	// Verify files were moved
	assert.NoFileExists(t, file1Path)
	assert.NoFileExists(t, file2Path)
	assert.FileExists(t, filepath.Join(subdirPath, "file1.txt"))
	assert.FileExists(t, filepath.Join(subdirPath, "file2.txt"))

	// Now test the undo operation
	cmdDetails, err := undoer.ParseGitCommand("git mv file1.txt file2.txt subdir/")
	require.NoError(t, err)

	// Create the real GitExec (not mock)
	realGitExec := githelpers.NewGitHelper(tmpDir)
	mvUndoer := undoer.NewMvUndoerForTest(realGitExec, cmdDetails)

	// Get the undo commands (should be multiple for multi-file mv)
	undoCommands, err := mvUndoer.GetUndoCommands()
	require.NoError(t, err)
	require.NotEmpty(t, undoCommands, "Should have at least one undo command")

	t.Logf("Number of undo commands: %d", len(undoCommands))
	for i, cmd := range undoCommands {
		t.Logf("Undo command %d: %s", i+1, cmd.Command)
		t.Logf("Undo description %d: %s", i+1, cmd.Description)
	}

	// Execute all undo commands in sequence
	for i, undoCmd := range undoCommands {
		err = undoCmd.Exec()
		if err != nil {
			t.Logf("Undo command %d failed: %v", i+1, err)
		}
		require.NoError(t, err, "Undo command %d should succeed", i+1)
	}

	// Verify files are back in original location
	assert.FileExists(t, file1Path, "file1.txt should be restored to original location")
	assert.FileExists(t, file2Path, "file2.txt should be restored to original location")
	assert.NoFileExists(t, filepath.Join(subdirPath, "file1.txt"), "file1.txt should not exist in subdir")
	assert.NoFileExists(t, filepath.Join(subdirPath, "file2.txt"), "file2.txt should not exist in subdir")

	// Verify file contents are preserved
	content1, err := os.ReadFile(file1Path)
	require.NoError(t, err)
	assert.Equal(t, "file1 content", string(content1))

	content2, err := os.ReadFile(file2Path)
	require.NoError(t, err)
	assert.Equal(t, "file2 content", string(content2))
}
