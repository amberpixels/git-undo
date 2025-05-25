package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.0", 0},
		{"v1.0.0", "v1.0.1", -1},
		{"1.0.0", "v1.0.1", -1},
		{"v1.0.1", "1.0.0", 1},
		{"dev", "1.0.0", -1},
		{"1.0.0", "dev", 1},
		{"dev", "dev", 0},
		{"1.2.3", "1.2.4", -1},
		{"1.2.4", "1.2.3", 1},
		{"1.2.3", "1.3.0", -1},
		{"1.3.0", "1.2.3", 1},
		{"2.0.0", "1.9.9", 1},
		{"1.9.9", "2.0.0", -1},
		{"1.0.0-beta", "1.0.0", 0}, // Base versions are same
		{"1.0", "1.0.0", 0},        // Missing patch version
	}

	for _, test := range tests {
		t.Run(test.v1+"_vs_"+test.v2, func(t *testing.T) {
			result := compareVersions(test.v1, test.v2)
			assert.Equal(t, test.expected, result, "compareVersions(%s, %s) = %d, expected %d", test.v1, test.v2, result, test.expected)
		})
	}
}

func TestAutoUpdateConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "git-undo-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a mock app with a temporary git directory
	app := &App{
		git: &mockGitHelper{gitDir: tempDir},
	}

	// Test loading non-existent config
	config, err := app.loadAutoUpdateConfig()
	require.NoError(t, err)
	assert.Equal(t, time.Time{}, config.LastCheckTime)
	assert.Equal(t, "", config.LastVersion)

	// Test saving and loading config
	now := time.Now()
	config.LastCheckTime = now
	config.LastVersion = "v1.2.3"

	err = app.saveAutoUpdateConfig(config)
	require.NoError(t, err)

	// Load the config back
	loadedConfig, err := app.loadAutoUpdateConfig()
	require.NoError(t, err)
	assert.Equal(t, now.Unix(), loadedConfig.LastCheckTime.Unix()) // Compare Unix timestamps to avoid precision issues
	assert.Equal(t, "v1.2.3", loadedConfig.LastVersion)

	// Verify the file was created in the correct location
	configPath, err := app.getAutoUpdateConfigPath()
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(configPath))

	// Check file exists and has correct content
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var fileConfig AutoUpdateConfig
	err = json.Unmarshal(data, &fileConfig)
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", fileConfig.LastVersion)
}

func TestAutoUpdateConfigPath(t *testing.T) {
	// Test with git directory
	tempGitDir, err := os.MkdirTemp("", "git-undo-test-git")
	require.NoError(t, err)
	defer os.RemoveAll(tempGitDir)

	app := &App{
		git: &mockGitHelper{gitDir: tempGitDir},
	}

	configPath, err := app.getAutoUpdateConfigPath()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tempGitDir, "git-undo-autoupdate.json"), configPath)

	// Test without git directory (should use global config)
	app = &App{
		git: &mockGitHelper{gitDirError: true},
	}

	configPath, err = app.getAutoUpdateConfigPath()
	require.NoError(t, err)

	homeDir, _ := os.UserHomeDir()
	expectedPath := filepath.Join(homeDir, ".config", "git-undo", "autoupdate.json")
	assert.Equal(t, expectedPath, configPath)
}

// mockGitHelper is a mock implementation of GitHelper for testing
type mockGitHelper struct {
	gitDir      string
	gitDirError bool
}

func (m *mockGitHelper) GetCurrentGitRef() (string, error) {
	return "main", nil
}

func (m *mockGitHelper) GetRepoGitDir() (string, error) {
	if m.gitDirError {
		return "", assert.AnError
	}
	return m.gitDir, nil
}

func (m *mockGitHelper) GitRun(subCmd string, args ...string) error {
	return nil
}

func (m *mockGitHelper) GitOutput(subCmd string, args ...string) (string, error) {
	return "", nil
}
