package logging

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/amberpixels/git-undo/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogger_E2E(t *testing.T) {
	// 1. Create a brand-new sandbox
	tmpGitUndoDir, err := os.MkdirTemp("", "gitundo-test-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tmpGitUndoDir)
	}()

	// Create logger instance
	logger := NewLogger()
	require.NotNil(t, logger)

	// Create and set mock git ref reader
	mgc := testutil.NewMockGitCtrl(tmpGitUndoDir)
	require.NoError(t, logger.Init(mgc))

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
		testutil.SwitchRef(mgc, cmd.ref) // Set mock ref before logging
		err := logger.LogCommand(cmd.cmd)
		require.NoError(t, err)
	}

	// Verify log file exists and has content
	logFile := logger.GetLogPath()
	_, err = os.Stat(logFile)
	require.NoError(t, err)

	// 2.1. Read all log entries
	content, err := logger.readLogFile()
	require.NoError(t, err)
	assert.NotEmpty(t, content)
	lines := strings.Split(string(content), "\n")
	fmt.Println("~~~ ", len(lines))

	// 2.2 Get latest entry from feature/test branch
	t.Log("Getting latest entry from feature/test...")
	testutil.SwitchRef(mgc, "feature/test")
	entry, err := logger.GetEntry(RegularEntry)
	require.NoError(t, err)
	assert.Equal(t, commands[4].cmd, entry.Command)
	assert.Equal(t, "feature/test", entry.Ref)

	// 3. Toggle the latest entry as undoed
	t.Log("Toggling latest entry as undoed...")
	marked, err := logger.ToggleEntry(entry.GetIdentifier())
	require.NoError(t, err)
	assert.True(t, marked, "Entry should be marked as undoed")

	// 4. Get the latest undoed entry
	t.Log("Getting latest undoed entry...")
	undoedEntry, err := logger.GetEntry(UndoedEntry)
	require.NoError(t, err)
	assert.Equal(t, entry.Command, undoedEntry.Command)
	assert.Equal(t, entry.Ref, undoedEntry.Ref)

	// 5. Toggle the entry back to regular
	t.Log("Toggling entry back to regular...")
	marked, err = logger.ToggleEntry(undoedEntry.GetIdentifier())
	require.NoError(t, err)
	assert.False(t, marked, "Entry should be unmarked")

	// 6. Switch to main branch and get its latest entry
	t.Log("Getting latest entry from main branch...")
	testutil.SwitchRef(mgc, "main")

	mainEntry, err := logger.GetEntry(RegularEntry)
	require.NoError(t, err)
	assert.Equal(t, commands[1].cmd, mainEntry.Command)
	assert.Equal(t, "main", mainEntry.Ref)

	// 7. Test entry parsing
	t.Log("Testing entry parsing...")
	parsedEntry, err := parseLogLine(mainEntry.GetIdentifier(), false)
	require.NoError(t, err)
	assert.Equal(t, mainEntry.Command, parsedEntry.Command)
	assert.Equal(t, mainEntry.Ref, parsedEntry.Ref)
	assert.WithinDuration(t, time.Now(), parsedEntry.Timestamp, 24*time.Hour)

	// 8. Test git undo command (should be skipped)
	t.Log("Testing git undo command logging...")
	err = logger.LogCommand("git undo")
	require.NoError(t, err)
	// Get latest entry - should still be the previous one
	latestEntry, err := logger.GetEntry(RegularEntry)
	require.NoError(t, err)
	assert.Equal(t, mainEntry.Command, latestEntry.Command)
}
