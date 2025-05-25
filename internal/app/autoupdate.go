package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// AutoUpdateCheckInterval defines how often to check for updates (7 days)
	AutoUpdateCheckInterval = 7 * 24 * time.Hour

	// GitHub API URL for checking latest release
	GitHubAPIURL = "https://api.github.com/repos/amberpixels/git-undo/releases/latest"
)

// AutoUpdateConfig stores the auto-update configuration and state
type AutoUpdateConfig struct {
	LastCheckTime time.Time `json:"last_check_time"`
	LastVersion   string    `json:"last_version"`
}

// GitHubRelease represents the GitHub API response for a release
type GitHubRelease struct {
	TagName string `json:"tag_name"`
}

// getAutoUpdateConfigPath returns the path to the auto-update config file
func (a *App) getAutoUpdateConfigPath() (string, error) {
	gitDir, err := a.git.GetRepoGitDir()
	if err != nil {
		// If we're not in a git repo, use a global config directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		configDir := filepath.Join(homeDir, ".config", "git-undo")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create config directory: %w", err)
		}
		return filepath.Join(configDir, "autoupdate.json"), nil
	}

	// Use git directory for repo-specific config
	configPath := filepath.Join(gitDir, "git-undo-autoupdate.json")
	return configPath, nil
}

// loadAutoUpdateConfig loads the auto-update configuration
func (a *App) loadAutoUpdateConfig() (*AutoUpdateConfig, error) {
	configPath, err := a.getAutoUpdateConfigPath()
	if err != nil {
		return nil, err
	}

	config := &AutoUpdateConfig{}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Config doesn't exist, return default config
			return config, nil
		}
		return nil, fmt.Errorf("failed to read auto-update config: %w", err)
	}

	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse auto-update config: %w", err)
	}

	return config, nil
}

// saveAutoUpdateConfig saves the auto-update configuration
func (a *App) saveAutoUpdateConfig(config *AutoUpdateConfig) error {
	configPath, err := a.getAutoUpdateConfigPath()
	if err != nil {
		return err
	}

	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal auto-update config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write auto-update config: %w", err)
	}

	return nil
}

// getLatestVersion fetches the latest version from GitHub API
func (a *App) getLatestVersion() (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(GitHubAPIURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return "", fmt.Errorf("failed to parse GitHub API response: %w", err)
	}

	return release.TagName, nil
}

// compareVersions compares two version strings
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	// Remove 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// Handle development versions
	if v1 == "dev" && v2 != "dev" {
		return -1 // dev is always older than any release
	}
	if v1 != "dev" && v2 == "dev" {
		return 1 // any release is newer than dev
	}
	if v1 == "dev" && v2 == "dev" {
		return 0 // dev == dev
	}

	// Split versions into parts
	parts1 := strings.Split(strings.Split(v1, "-")[0], ".")
	parts2 := strings.Split(strings.Split(v2, "-")[0], ".")

	// Ensure both have at least 3 parts (major.minor.patch)
	for len(parts1) < 3 {
		parts1 = append(parts1, "0")
	}
	for len(parts2) < 3 {
		parts2 = append(parts2, "0")
	}

	// Compare each part
	for i := 0; i < 3; i++ {
		var n1, n2 int
		fmt.Sscanf(parts1[i], "%d", &n1)
		fmt.Sscanf(parts2[i], "%d", &n2)

		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}

	return 0
}

// checkForUpdates checks if an update is available and prompts the user
func (a *App) checkForUpdates() {
	// Load auto-update config
	config, err := a.loadAutoUpdateConfig()
	if err != nil {
		a.logDebugf("Failed to load auto-update config: %v", err)
		return
	}

	// Check if enough time has passed since last check
	if time.Since(config.LastCheckTime) < AutoUpdateCheckInterval {
		a.logDebugf("Auto-update check skipped (last check: %v)", config.LastCheckTime.Format("2006-01-02 15:04:05"))
		return
	}

	a.logDebugf("Checking for updates...")

	// Get latest version from GitHub
	latestVersion, err := a.getLatestVersion()
	if err != nil {
		a.logDebugf("Failed to check for updates: %v", err)
		// Update last check time even if failed to avoid spamming
		config.LastCheckTime = time.Now()
		_ = a.saveAutoUpdateConfig(config)
		return
	}

	// Update last check time and version
	config.LastCheckTime = time.Now()
	config.LastVersion = latestVersion
	if err := a.saveAutoUpdateConfig(config); err != nil {
		a.logDebugf("Failed to save auto-update config: %v", err)
	}

	// Compare with current version
	currentVersion := a.buildVersion
	if compareVersions(currentVersion, latestVersion) < 0 {
		// Update available
		fmt.Fprintf(os.Stderr, "\n"+yellowColor+"🔄 Update available: %s → %s"+resetColor+"\n", currentVersion, latestVersion)
		fmt.Fprintf(os.Stderr, grayColor+"Run 'git undo self-update' to update"+resetColor+"\n\n")
	} else {
		a.logDebugf("No update available (current: %s, latest: %s)", currentVersion, latestVersion)
	}
}

// AutoUpdate performs the auto-update check if needed
func (a *App) AutoUpdate() {
	// Only check for updates in normal operation, not for self-management commands
	// and not in verbose/dry-run modes to avoid noise
	if a.verbose || a.dryRun {
		return
	}

	// Run in background to avoid blocking the main operation
	go a.checkForUpdates()
}
