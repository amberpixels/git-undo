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

// TestCommitQuoteNormalizationIssue reproduces the bug where git commit commands
// with different quote patterns create duplicate log entries.
func TestCommitQuoteNormalizationIssue(t *testing.T) {
	t.Log("Testing commit command quote normalization issue")

	// Commands that should normalize to the same thing
	quotedCmd := `git commit -m "Add file2.txt"`
	unquotedCmd := `git commit -m Add file2.txt`

	t.Logf("Quoted command: %s", quotedCmd)
	t.Logf("Unquoted command: %s", unquotedCmd)

	// Test normalization
	quotedNorm := testNormalizeCommand(t, quotedCmd)
	unquotedNorm := testNormalizeCommand(t, unquotedCmd)

	t.Logf("Quoted normalized: %s", quotedNorm)
	t.Logf("Unquoted normalized: %s", unquotedNorm)

	// They should normalize to the same thing
	assert.Equal(t, quotedNorm, unquotedNorm, "Both commands should normalize to the same form")

	// Test with more complex scenarios
	testCases := []struct {
		name     string
		commands []string
	}{
		{
			name: "Basic commit messages",
			commands: []string{
				`git commit -m "test message"`,
				`git commit -m 'test message'`,
				`git commit -m test message`,
			},
		},
		{
			name: "Messages with spaces",
			commands: []string{
				`git commit -m "Add file2.txt"`,
				`git commit -m 'Add file2.txt'`,
				`git commit -m Add file2.txt`,
			},
		},
		{
			name: "Verbose flag variations",
			commands: []string{
				`git commit --verbose -m "commit f2"`,
				`git commit -m "commit f2"`,
				`git commit -m commit f2`,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var normalizedForms []string
			for _, cmd := range tc.commands {
				norm := testNormalizeCommand(t, cmd)
				normalizedForms = append(normalizedForms, norm)
				t.Logf("Command: %s -> Normalized: %s", cmd, norm)
			}

			// All normalized forms should be identical
			for i := 1; i < len(normalizedForms); i++ {
				assert.Equal(t, normalizedForms[0], normalizedForms[i],
					"Commands should normalize to the same form")
			}
		})
	}
}

// TestActualDuplicateLogging tests that deduplication actually works in practice.
func TestActualDuplicateLogging(t *testing.T) {
	t.Log("Testing actual duplicate logging scenario that reproduces BATS failure")

	mgc := NewMockGitHelper()
	SwitchRef(mgc, "main")

	tmpDir := t.TempDir()
	lgr := logging.NewLogger(tmpDir, mgc)
	require.NotNil(t, lgr)

	// Test the exact scenario from the BATS test output:
	// One command has quotes, one doesn't, but they represent the same git operation
	quotedCmd := `git commit -m "Add file2.txt"`
	unquotedCmd := `git commit -m Add file2.txt`

	t.Logf("Testing commands:")
	t.Logf("  1. %s", quotedCmd)
	t.Logf("  2. %s", unquotedCmd)

	// Simulate different hook environments
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

	// First: Git hook logs the quoted version
	t.Setenv("GIT_UNDO_GIT_HOOK_MARKER", "1")
	t.Setenv("GIT_UNDO_INTERNAL_HOOK", "1")
	t.Setenv("GIT_HOOK_NAME", "post-commit")

	err := lgr.LogCommand(quotedCmd)
	require.NoError(t, err)

	// Second: Shell hook logs the unquoted version
	t.Setenv("GIT_UNDO_GIT_HOOK_MARKER", "")
	t.Setenv("GIT_UNDO_INTERNAL_HOOK", "1")
	t.Setenv("GIT_HOOK_NAME", "")

	err = lgr.LogCommand(unquotedCmd)
	require.NoError(t, err)

	// Check log content - should have only ONE entry due to deduplication
	var buffer bytes.Buffer
	require.NoError(t, lgr.Dump(&buffer))
	content := buffer.String()

	t.Logf("Log content:\n%s", content)

	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = []string{} // Empty file
	}

	// This test should FAIL initially, showing the bug
	// After we fix it, this should pass
	assert.Len(t, lines, 1, "Should have exactly 1 log entry due to deduplication, but got: %v", lines)

	if len(lines) > 1 {
		t.Logf("BUG REPRODUCED: Found %d entries when expecting 1:", len(lines))
		for i, line := range lines {
			t.Logf("  Entry %d: %s", i+1, line)
		}
	}
}

// TestGitBackToggleBehavior tests that git-back can toggle back and forth
// between branches even after its own undoed entries.
func TestGitBackToggleBehavior(t *testing.T) {
	t.Log("Testing git-back toggle behavior with undoed checkout entries")

	mgc := NewMockGitHelper()
	SwitchRef(mgc, "another-branch")

	tmpDir := t.TempDir()
	lgr := logging.NewLogger(tmpDir, mgc)
	require.NotNil(t, lgr)

	// Simulate the exact scenario from the failing BATS test:
	// Start on another-branch, simulate multiple git-back calls that mark checkouts as undoed

	// 1. First checkout to feature-branch
	err := lgr.LogCommand("git checkout feature-branch")
	require.NoError(t, err)
	SwitchRef(mgc, "feature-branch")

	// 2. git-back call 1: Find and mark the checkout as undoed
	entry1, err := lgr.GetLastCheckoutSwitchEntry(logging.RefAny)
	require.NoError(t, err)
	require.NotNil(t, entry1)
	assert.Equal(t, "git checkout feature-branch", entry1.Command)

	err = lgr.ToggleEntry(entry1.GetIdentifier())
	require.NoError(t, err)
	SwitchRef(mgc, "another-branch")

	// 3. Checkout to main
	err = lgr.LogCommand("git checkout main")
	require.NoError(t, err)
	SwitchRef(mgc, "main")

	// 4. git-back call 2: Should find the checkout to main and mark it as undoed
	entry2, err := lgr.GetLastCheckoutSwitchEntry(logging.RefAny)
	require.NoError(t, err)
	require.NotNil(t, entry2)
	assert.Equal(t, "git checkout main", entry2.Command)

	err = lgr.ToggleEntry(entry2.GetIdentifier())
	require.NoError(t, err)
	SwitchRef(mgc, "another-branch")

	// 5. Add some other activity to make the log more complex
	err = lgr.LogCommand("git add unstaged.txt")
	require.NoError(t, err)

	// 6. Current implementation fails
	entry3, err := lgr.GetLastCheckoutSwitchEntry(logging.RefAny)
	require.NoError(t, err)
	assert.Nil(t, entry3, "Current implementation should fail to find checkout when all are undoed")

	// 7. But new method should work
	entry4, err := lgr.GetLastCheckoutSwitchEntryForToggle(logging.RefAny)
	require.NoError(t, err)
	assert.NotNil(t, entry4, "New method should find checkout command even if undoed")
	if entry4 != nil {
		t.Logf("Found checkout entry: %s (undoed: %v)", entry4.Command, entry4.Undoed)
	}

	// Check the log to see what's in there
	var buffer bytes.Buffer
	require.NoError(t, lgr.Dump(&buffer))
	content := buffer.String()
	t.Logf("Log content:\n%s", content)
}

// TestGitBackFindAnyCheckout tests that git-back can find checkout commands
// regardless of their undoed status.
func TestGitBackFindAnyCheckout(t *testing.T) {
	t.Log("Testing git-back can find any checkout command for toggle behavior")

	mgc := NewMockGitHelper()
	SwitchRef(mgc, "main")

	tmpDir := t.TempDir()
	lgr := logging.NewLogger(tmpDir, mgc)
	require.NotNil(t, lgr)

	// Simple scenario: log some checkouts and test finding them
	err := lgr.LogCommand("git checkout feature-1")
	require.NoError(t, err)

	err = lgr.LogCommand("git checkout feature-2")
	require.NoError(t, err)

	err = lgr.LogCommand("git add file.txt") // Non-checkout command
	require.NoError(t, err)

	// Test the new method can find the latest checkout
	entry, err := lgr.GetLastCheckoutSwitchEntryForToggle(logging.RefAny)
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, "git checkout feature-2", entry.Command)
	assert.False(t, entry.Undoed)

	// Mark it as undoed
	err = lgr.ToggleEntry(entry.GetIdentifier())
	require.NoError(t, err)

	// Traditional method should not find it now
	entry2, err := lgr.GetLastCheckoutSwitchEntry(logging.RefAny)
	require.NoError(t, err)
	require.NotNil(t, entry2)
	assert.Equal(t, "git checkout feature-1", entry2.Command) // Should find the previous one

	// But new method should still find the latest one (even though undoed)
	entry3, err := lgr.GetLastCheckoutSwitchEntryForToggle(logging.RefAny)
	require.NoError(t, err)
	require.NotNil(t, entry3)
	assert.Equal(t, "git checkout feature-2", entry3.Command)
	assert.True(t, entry3.Undoed) // Should be marked as undoed

	t.Log("✅ git-back can successfully find checkout commands for toggle behavior")
}
