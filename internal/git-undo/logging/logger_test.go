package logging_test

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/amberpixels/git-undo/internal/git-undo/logging"
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
		currentRef: "main",
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
	assert.Equal(t, "feature/test", entry.Ref)

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
	SwitchRef(mgc, "main")

	mainEntry, err := lgr.GetLastRegularEntry()
	require.NoError(t, err)
	assert.Equal(t, commands[1].cmd, mainEntry.Command)
	assert.Equal(t, "main", mainEntry.Ref)

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
