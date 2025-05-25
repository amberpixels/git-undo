package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoUpdateIntegration(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "git-undo-integration-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a mock app with a temporary git directory
	app := &App{
		buildVersion: "v1.0.0", // Simulate an older version
		verbose:      false,
		dryRun:       false,
		git:          &mockGitHelper{gitDir: tempDir},
	}

	// Test 1: First run should trigger update check (but won't show notification due to network call)
	app.AutoUpdate()

	// Give the goroutine a moment to complete
	time.Sleep(100 * time.Millisecond)

	// Verify config file was created
	configPath := filepath.Join(tempDir, "git-undo-autoupdate.json")
	_, err = os.Stat(configPath)
	assert.NoError(t, err, "Auto-update config file should be created")

	// Test 2: Load the config and verify it was updated
	config, err := app.loadAutoUpdateConfig()
	require.NoError(t, err)

	// The last check time should be recent (within the last minute)
	assert.True(t, time.Since(config.LastCheckTime) < time.Minute,
		"Last check time should be recent, got: %v", config.LastCheckTime)

	// Test 3: Immediate second call should skip the check
	oldCheckTime := config.LastCheckTime
	app.AutoUpdate()
	time.Sleep(100 * time.Millisecond)

	// Load config again
	config, err = app.loadAutoUpdateConfig()
	require.NoError(t, err)

	// Check time should be the same (no new check performed)
	assert.Equal(t, oldCheckTime.Unix(), config.LastCheckTime.Unix(),
		"Second check should be skipped due to recent check")

	// Test 4: Simulate old check time to trigger new check
	config.LastCheckTime = time.Now().Add(-8 * 24 * time.Hour) // 8 days ago
	err = app.saveAutoUpdateConfig(config)
	require.NoError(t, err)

	app.AutoUpdate()
	time.Sleep(100 * time.Millisecond)

	// Load config again
	newConfig, err := app.loadAutoUpdateConfig()
	require.NoError(t, err)

	// Check time should be updated
	assert.True(t, newConfig.LastCheckTime.After(config.LastCheckTime),
		"Check time should be updated after old timestamp")
}

func TestAutoUpdateSkipsInVerboseMode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "git-undo-verbose-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	app := &App{
		buildVersion: "v1.0.0",
		verbose:      true, // Verbose mode should skip auto-update
		dryRun:       false,
		git:          &mockGitHelper{gitDir: tempDir},
	}

	app.AutoUpdate()
	time.Sleep(100 * time.Millisecond)

	// Config file should not be created in verbose mode
	configPath := filepath.Join(tempDir, "git-undo-autoupdate.json")
	_, err = os.Stat(configPath)
	assert.True(t, os.IsNotExist(err), "Auto-update config should not be created in verbose mode")
}

func TestAutoUpdateSkipsInDryRunMode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "git-undo-dryrun-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	app := &App{
		buildVersion: "v1.0.0",
		verbose:      false,
		dryRun:       true, // Dry-run mode should skip auto-update
		git:          &mockGitHelper{gitDir: tempDir},
	}

	app.AutoUpdate()
	time.Sleep(100 * time.Millisecond)

	// Config file should not be created in dry-run mode
	configPath := filepath.Join(tempDir, "git-undo-autoupdate.json")
	_, err = os.Stat(configPath)
	assert.True(t, os.IsNotExist(err), "Auto-update config should not be created in dry-run mode")
}
