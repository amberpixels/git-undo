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
	require.NotNil(t, entry)
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
	parsedEntry, err := logging.ParseLogLine(mainEntry.String())
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

// TestBranchTruncation tests the branch-aware log truncation functionality.
func TestBranchTruncation(t *testing.T) {
	t.Log("Testing branch truncation logic when logging after undos")

	mgc := NewMockGitHelper()
	SwitchRef(mgc, "main")

	tmpDir := t.TempDir()
	lgr := logging.NewLogger(tmpDir, mgc)
	require.NotNil(t, lgr)

	// Set up the scenario: A → B → C → undo → undo → F
	// Log commands A, B, C
	err := lgr.LogCommand("git add fileA.txt")
	require.NoError(t, err)
	err = lgr.LogCommand("git commit -m 'B'")
	require.NoError(t, err)
	err = lgr.LogCommand("git add fileC.txt")
	require.NoError(t, err)

	// Get and undo C
	entryC, err := lgr.GetLastRegularEntry()
	require.NoError(t, err)
	require.NotNil(t, entryC)
	assert.Equal(t, "git add fileC.txt", entryC.Command)
	err = lgr.ToggleEntry(entryC.GetIdentifier())
	require.NoError(t, err)

	// Get and undo B
	entryB, err := lgr.GetLastRegularEntry()
	require.NoError(t, err)
	require.NotNil(t, entryB)
	assert.Equal(t, "git commit -m 'B'", entryB.Command)
	err = lgr.ToggleEntry(entryB.GetIdentifier())
	require.NoError(t, err)

	// Check that we have 2 consecutive undone commands
	count, err := lgr.CountConsecutiveUndoneCommands()
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Now log command F - this should trigger branch truncation
	err = lgr.LogCommand("git add fileF.txt")
	require.NoError(t, err)

	// After truncation, the log should contain only F and A
	var buffer bytes.Buffer
	require.NoError(t, lgr.Dump(&buffer))
	content := buffer.String()
	t.Logf("Log content after truncation:\n%s", content)

	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = []string{}
	}

	assert.Len(t, lines, 2, "After branching, should have 2 entries (F and A)")

	// Verify the entries are correct
	entryF, err := lgr.GetLastRegularEntry()
	require.NoError(t, err)
	require.NotNil(t, entryF)
	assert.Equal(t, "git add fileF.txt", entryF.Command)

	t.Log("✅ Branch truncation working correctly")
}

// TestNavigationPrefixing tests that navigation commands are prefixed with N.
func TestNavigationPrefixing(t *testing.T) {
	t.Log("Testing navigation command prefixing with N")

	mgc := NewMockGitHelper()
	SwitchRef(mgc, "main")

	tmpDir := t.TempDir()
	lgr := logging.NewLogger(tmpDir, mgc)
	require.NotNil(t, lgr)

	// Log a navigation command
	err := lgr.LogCommand("git checkout feature")
	require.NoError(t, err)

	// Log a mutation command
	err = lgr.LogCommand("git add file.txt")
	require.NoError(t, err)

	// Check the raw log content
	var buffer bytes.Buffer
	require.NoError(t, lgr.Dump(&buffer))
	content := buffer.String()
	t.Logf("Log content:\n%s", content)

	lines := strings.Split(strings.TrimSpace(content), "\n")
	require.Len(t, lines, 2)

	// First line should be mutation command (+M prefix)
	assert.Contains(t, lines[0], "+M ")
	assert.Contains(t, lines[0], "git add file.txt")

	// Second line should be navigation command (+N prefix)
	assert.Contains(t, lines[1], "+N ")
	assert.Contains(t, lines[1], "git checkout feature")

	t.Log("✅ Navigation command prefixing working correctly")
}

// TestOldFormatMigration tests that old format files are truncated during migration.
func TestOldFormatMigration(t *testing.T) {
	t.Log("Testing old format migration (truncation)")

	mgc := NewMockGitHelper()
	SwitchRef(mgc, "main")

	tmpDir := t.TempDir()

	// Manually create a log with old format (no +/- M/N prefixes)
	logPath := tmpDir + "/.git/git-undo/commands"
	err := os.MkdirAll(tmpDir+"/.git/git-undo", 0755)
	require.NoError(t, err)

	oldFormatContent := `2025-06-25 10:00:00|main|git add old-file.txt
N 2025-06-25 09:59:00|main|git checkout old-branch
2025-06-25 09:58:00|main|git commit -m 'old commit'`

	err = os.WriteFile(logPath, []byte(oldFormatContent), 0600)
	require.NoError(t, err)

	// Create logger - this should trigger migration (truncation)
	lgr := logging.NewLogger(tmpDir+"/.git", mgc)
	require.NotNil(t, lgr)

	// Check that the file was truncated (should be empty)
	var buffer bytes.Buffer
	require.NoError(t, lgr.Dump(&buffer))
	content := buffer.String()
	assert.Empty(t, strings.TrimSpace(content), "Old format file should be truncated")

	// Now log new commands which should use new format
	err = lgr.LogCommand("git checkout new-branch")
	require.NoError(t, err)
	err = lgr.LogCommand("git add new-file.txt")
	require.NoError(t, err)

	// Check log content has new format
	buffer.Reset()
	require.NoError(t, lgr.Dump(&buffer))
	content = buffer.String()
	t.Logf("Log content after migration:\n%s", content)

	lines := strings.Split(strings.TrimSpace(content), "\n")
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], "+M ", "Should use new format +M")
	assert.Contains(t, lines[1], "+N ", "Should use new format +N")

	t.Log("✅ Old format migration working correctly")
}

// TestNavigationCommandSeparation tests that git-undo and git-back handle commands separately.
func TestNavigationCommandSeparation(t *testing.T) {
	t.Log("Testing navigation command separation for git-undo vs git-back")

	mgc := NewMockGitHelper()
	SwitchRef(mgc, "main")

	tmpDir := t.TempDir()
	lgr := logging.NewLogger(tmpDir, mgc)
	require.NotNil(t, lgr)

	// Log a mix of navigation and mutation commands
	err := lgr.LogCommand("git checkout feature")
	require.NoError(t, err)
	err = lgr.LogCommand("git add file1.txt")
	require.NoError(t, err)
	err = lgr.LogCommand("git switch main")
	require.NoError(t, err)
	err = lgr.LogCommand("git commit -m 'test'")
	require.NoError(t, err)

	// git-undo should get the last mutation command (commit)
	undoEntry, err := lgr.GetLastRegularEntry()
	require.NoError(t, err)
	require.NotNil(t, undoEntry)
	assert.Equal(t, "git commit -m 'test'", undoEntry.Command)

	// git-back should get the last navigation command (switch)
	backEntry, err := lgr.GetLastCheckoutSwitchEntry()
	require.NoError(t, err)
	require.NotNil(t, backEntry)
	assert.Equal(t, "git switch main", backEntry.Command)

	t.Log("✅ Navigation command separation working correctly")
}

// TestTruncateToCurrentBranchPreservesNavigation tests that truncation preserves navigation commands.
func TestTruncateToCurrentBranchPreservesNavigation(t *testing.T) {
	t.Log("Testing that branch truncation preserves all navigation commands")

	mgc := NewMockGitHelper()
	SwitchRef(mgc, "main")

	tmpDir := t.TempDir()
	lgr := logging.NewLogger(tmpDir, mgc)
	require.NotNil(t, lgr)

	// Create a complex scenario with mixed navigation and mutation commands
	err := lgr.LogCommand("git checkout feature") // N prefixed
	require.NoError(t, err)
	err = lgr.LogCommand("git add fileA.txt") // mutation
	require.NoError(t, err)
	err = lgr.LogCommand("git switch main") // N prefixed
	require.NoError(t, err)
	err = lgr.LogCommand("git commit -m 'B'") // mutation
	require.NoError(t, err)
	err = lgr.LogCommand("git add fileC.txt") // mutation
	require.NoError(t, err)

	// Undo the last two mutation commands
	entryC, err := lgr.GetLastRegularEntry()
	require.NoError(t, err)
	err = lgr.ToggleEntry(entryC.GetIdentifier())
	require.NoError(t, err)

	entryB, err := lgr.GetLastRegularEntry()
	require.NoError(t, err)
	err = lgr.ToggleEntry(entryB.GetIdentifier())
	require.NoError(t, err)

	// Manually call truncation
	err = lgr.TruncateToCurrentBranch()
	require.NoError(t, err)

	// Check that navigation commands are preserved
	var buffer bytes.Buffer
	require.NoError(t, lgr.Dump(&buffer))
	content := buffer.String()
	t.Logf("Log content after truncation:\n%s", content)

	// Should have both navigation commands plus the remaining mutation command
	navEntry1, err := lgr.GetLastCheckoutSwitchEntry()
	require.NoError(t, err)
	require.NotNil(t, navEntry1)
	assert.Equal(t, "git switch main", navEntry1.Command)

	// Verify navigation history is intact for git-back
	lines := strings.Split(strings.TrimSpace(content), "\n")
	navigationLines := 0
	for _, line := range lines {
		if strings.Contains(line, "+N ") || strings.Contains(line, "-N ") {
			navigationLines++
		}
	}
	assert.Equal(t, 2, navigationLines, "Should preserve both navigation commands")

	t.Log("✅ Branch truncation preserves navigation commands correctly")
}

// TestGetLastUndoedEntry tests the GetLastUndoedEntry method for redo functionality.
func TestGetLastUndoedEntry(t *testing.T) {
	t.Log("Testing GetLastUndoedEntry method for redo functionality")

	mgc := NewMockGitHelper()
	SwitchRef(mgc, "main")

	tmpDir := t.TempDir()
	lgr := logging.NewLogger(tmpDir, mgc)
	require.NotNil(t, lgr)

	// Log commands A, B, C
	err := lgr.LogCommand("git add fileA.txt")
	require.NoError(t, err)
	err = lgr.LogCommand("git add fileB.txt")
	require.NoError(t, err)
	err = lgr.LogCommand("git add fileC.txt")
	require.NoError(t, err)

	// Initially, no undoed entries
	undoedEntry, err := lgr.GetLastUndoedEntry()
	require.NoError(t, err)
	assert.Nil(t, undoedEntry)

	// Get and undo C
	entryC, err := lgr.GetLastRegularEntry()
	require.NoError(t, err)
	err = lgr.ToggleEntry(entryC.GetIdentifier())
	require.NoError(t, err)

	// Now should find C as last undoed entry
	undoedEntry, err = lgr.GetLastUndoedEntry()
	require.NoError(t, err)
	require.NotNil(t, undoedEntry)
	assert.Equal(t, "git add fileC.txt", undoedEntry.Command)
	assert.True(t, undoedEntry.Undoed)

	// Get and undo B
	entryB, err := lgr.GetLastRegularEntry()
	require.NoError(t, err)
	err = lgr.ToggleEntry(entryB.GetIdentifier())
	require.NoError(t, err)

	// Now should still find C as last undoed entry (C is at top of log)
	undoedEntry, err = lgr.GetLastUndoedEntry()
	require.NoError(t, err)
	require.NotNil(t, undoedEntry)
	assert.Equal(t, "git add fileC.txt", undoedEntry.Command)
	assert.True(t, undoedEntry.Undoed)

	// Test with navigation commands - should skip them
	err = lgr.LogCommand("git checkout feature")
	require.NoError(t, err)

	// Should still find C as last undoed entry (ignoring navigation commands)
	undoedEntry, err = lgr.GetLastUndoedEntry()
	require.NoError(t, err)
	require.NotNil(t, undoedEntry)
	assert.Equal(t, "git add fileC.txt", undoedEntry.Command)

	t.Log("✅ GetLastUndoedEntry working correctly for redo functionality")
}
