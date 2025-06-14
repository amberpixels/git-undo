package logging_test

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/amberpixels/git-undo/internal/git-undo/logging"
	"github.com/amberpixels/git-undo/internal/githelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockGitRefSwitcher implements GitHelper for testing ref switching.
type MockGitRefSwitcher struct {
	currentRef string
}

func (m *MockGitRefSwitcher) GetCurrentGitRef() (string, error) {
	return m.currentRef, nil
}

func (m *MockGitRefSwitcher) SwitchRef(ref string) {
	m.currentRef = ref
}

func NewMockGitHelper() *MockGitRefSwitcher {
	return &MockGitRefSwitcher{
		currentRef: logging.RefMain.String(),
	}
}

func SwitchRef(gh logging.GitHelper, ref string) {
	gh.(*MockGitRefSwitcher).SwitchRef(ref)
}

func TestLogger_E2E(t *testing.T) {
	// 1. Create a brand-new sandbox
	tmpGitUndoDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tmpGitUndoDir)
	}()

	// Create lgr instance
	mgc := NewMockGitHelper()
	lgr := logging.NewLogger(tmpGitUndoDir+"/.git", mgc)
	require.NotNil(t, lgr)

	// Test data - commands to log
	commands := []struct {
		cmd string
		ref string
	}{
		{cmd: "git commit -m \"Initial commit\"", ref: "main"},
		{cmd: "git branch feature/test", ref: "main"},
		{cmd: "git checkout feature/test", ref: "feature/test"},
		{cmd: "git commit -m 'Add test file'", ref: "feature/test"},
		{cmd: "git commit --amend -m \"Update test file\"", ref: "feature/test"},
	}

	// 1. Log all commands
	t.Log("Logging commands...")
	for _, cmd := range commands {
		SwitchRef(mgc, cmd.ref) // Set mock ref before logging
		err := lgr.LogCommand(cmd.cmd)
		require.NoError(t, err)
	}

	// Verify log file exists and has content
	logFile := lgr.GetLogPath()
	_, err := os.Stat(logFile)
	require.NoError(t, err)

	// 2.1. Read all log entries
	var buffer bytes.Buffer
	require.NoError(t, lgr.Dump(&buffer))
	content := buffer.Bytes()
	assert.NotEmpty(t, content)
	lines := strings.Split(string(content), "\n")
	assert.Len(t, lines, len(commands)+1)

	// 2.2 Get latest entry from feature/test branch
	t.Log("Getting latest entry from feature/test...")
	SwitchRef(mgc, "feature/test")
	entry, err := lgr.GetLastRegularEntry()
	require.NoError(t, err)
	assert.Equal(t, commands[4].cmd, entry.Command)
	assert.Equal(t, "feature/test", entry.Ref.String())

	// 3. Toggle the latest entry as undoed
	t.Log("Toggling latest entry as undoed...")
	require.NoError(t, lgr.ToggleEntry(entry.GetIdentifier()))

	// 4. Get the latest entry
	t.Log("Getting latest entry...")
	latestEntry, err := lgr.GetLastEntry()
	require.NoError(t, err)
	assert.Equal(t, entry.Command, latestEntry.Command)
	assert.Equal(t, entry.Ref, latestEntry.Ref)

	// 5. Toggle the entry back to regular
	t.Log("Toggling entry back to regular...")
	require.NoError(t, lgr.ToggleEntry(latestEntry.GetIdentifier()))

	// 6. Switch to main branch and get its latest entry
	t.Log("Getting latest entry from main branch...")
	SwitchRef(mgc, logging.RefMain.String())

	mainEntry, err := lgr.GetLastRegularEntry()
	require.NoError(t, err)
	assert.Equal(t, commands[1].cmd, mainEntry.Command)
	assert.Equal(t, logging.RefMain, mainEntry.Ref)

	// 7. Test entry parsing
	t.Log("Testing entry parsing...")
	parsedEntry, err := logging.ParseLogLine(mainEntry.GetIdentifier())
	require.NoError(t, err)
	assert.Equal(t, mainEntry.Command, parsedEntry.Command)
	assert.Equal(t, mainEntry.Ref, parsedEntry.Ref)
	assert.False(t, parsedEntry.Undoed)
	assert.WithinDuration(t, time.Now(), parsedEntry.Timestamp, 24*time.Hour)

	// 8. Test git undo command (should be skipped)
	t.Log("Testing git undo command logging...")
	err = lgr.LogCommand("git undo")
	require.NoError(t, err)

	// Get latest entry - should still be the previous one
	latestRegularEntry, err := lgr.GetLastRegularEntry()
	require.NoError(t, err)
	assert.Equal(t, mainEntry.Command, latestRegularEntry.Command)
}

// TestDeduplication tests the deduplication logic between shell and git hooks.
func TestDeduplication(t *testing.T) {
	t.Log("Testing deduplication between shell and git hooks")

	mgc := NewMockGitHelper()
	SwitchRef(mgc, "feature/make-linter-happy")

	tmpDir := t.TempDir()

	lgr := logging.NewLogger(tmpDir, mgc)
	require.NotNil(t, lgr)

	// Test the exact scenario from user's report
	shellCmd := `git commit --verbose -m 'cleanup'`
	gitCmd := `git commit -m "cleanup"`

	t.Logf("Shell command: %s", shellCmd)
	t.Logf("Git command: %s", gitCmd)

	// Test normalization first
	shellNorm := testNormalizeCommand(t, shellCmd)
	gitNorm := testNormalizeCommand(t, gitCmd)

	t.Logf("Shell normalized: %s", shellNorm)
	t.Logf("Git normalized: %s", gitNorm)

	assert.Equal(t, shellNorm, gitNorm, "Normalized commands should be equal")

	// Test multiple scenarios
	testScenarios := []struct {
		name        string
		gitFirst    bool
		expectCount int
	}{
		{"Git hook first (normal case)", true, 1},
		{"Shell hook first (race condition)", false, 1}, // Fixed: should also be 1 now
	}

	for _, tc := range testScenarios {
		t.Run(tc.name, func(t *testing.T) {
			// Clean up any previous test state
			_ = os.RemoveAll(tmpDir)
			err := os.MkdirAll(tmpDir, 0755)
			require.NoError(t, err)

			lgr := logging.NewLogger(tmpDir, mgc)
			require.NotNil(t, lgr)

			// Simulate hook environment
			oldMarker := os.Getenv("GIT_UNDO_GIT_HOOK_MARKER")
			oldInternal := os.Getenv("GIT_UNDO_INTERNAL_HOOK")
			oldHookName := os.Getenv("GIT_HOOK_NAME")
			defer func() {
				// Restore environment
				if oldMarker != "" {
					t.Setenv("GIT_UNDO_GIT_HOOK_MARKER", oldMarker)
				} else {
					os.Unsetenv("GIT_UNDO_GIT_HOOK_MARKER")
				}
				if oldInternal != "" {
					t.Setenv("GIT_UNDO_INTERNAL_HOOK", oldInternal)
				} else {
					os.Unsetenv("GIT_UNDO_INTERNAL_HOOK")
				}
				if oldHookName != "" {
					t.Setenv("GIT_HOOK_NAME", oldHookName)
				} else {
					os.Unsetenv("GIT_HOOK_NAME")
				}
			}()

			if tc.gitFirst {
				// Git hook logs first
				t.Setenv("GIT_UNDO_GIT_HOOK_MARKER", "1")
				t.Setenv("GIT_UNDO_INTERNAL_HOOK", "1")
				t.Setenv("GIT_HOOK_NAME", "post-commit")

				err = lgr.LogCommand(gitCmd)
				require.NoError(t, err)

				// Shell hook logs second
				t.Setenv("GIT_UNDO_GIT_HOOK_MARKER", "")
				t.Setenv("GIT_UNDO_INTERNAL_HOOK", "1")
				t.Setenv("GIT_HOOK_NAME", "")

				err = lgr.LogCommand(shellCmd)
				require.NoError(t, err)
			} else {
				// Shell hook logs first
				t.Setenv("GIT_UNDO_GIT_HOOK_MARKER", "")
				t.Setenv("GIT_UNDO_INTERNAL_HOOK", "1")
				t.Setenv("GIT_HOOK_NAME", "")

				err = lgr.LogCommand(shellCmd)
				require.NoError(t, err)

				// Git hook logs second
				t.Setenv("GIT_UNDO_GIT_HOOK_MARKER", "1")
				t.Setenv("GIT_UNDO_INTERNAL_HOOK", "1")
				t.Setenv("GIT_HOOK_NAME", "post-commit")

				err = lgr.LogCommand(gitCmd)
				require.NoError(t, err)
			}

			// Check log content
			var buffer bytes.Buffer
			require.NoError(t, lgr.Dump(&buffer))
			content := buffer.String()

			t.Logf("Log content:\n%s", content)

			lines := strings.Split(strings.TrimSpace(content), "\n")
			if len(lines) == 1 && lines[0] == "" {
				lines = []string{} // Empty file
			}

			t.Logf("Expected %d entries, got %d", tc.expectCount, len(lines))

			// Both scenarios should now result in exactly one entry
			assert.Len(
				t,
				lines,
				tc.expectCount,
				"Should have exactly %d log entry for %s, got: %v",
				tc.expectCount,
				tc.name,
				lines,
			)
		})
	}
}

// testNormalizeCommand is a helper to test command normalization.
func testNormalizeCommand(t *testing.T, cmd string) string {
	gitCmd, err := githelpers.ParseGitCommand(cmd)
	require.NoError(t, err)

	normalized, err := gitCmd.NormalizedString()
	if err != nil {
		return cmd // Return original if normalization fails
	}

	return normalized
}
