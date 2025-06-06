package githelpers_test

import (
	"reflect"
	"testing"

	"github.com/amberpixels/git-undo/internal/githelpers"
	"github.com/stretchr/testify/assert"
)

func TestIsReadOnlyGitCommand(t *testing.T) {
	// Test cases structure
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		// Empty and invalid commands
		{
			name:     "Empty command",
			command:  "",
			expected: false,
		},
		{
			name:     "Just git without subcommand",
			command:  "git",
			expected: false,
		},

		// Always read-only commands
		{
			name:     "Status command",
			command:  "git status",
			expected: true,
		},
		{
			name:     "Log command",
			command:  "git log",
			expected: true,
		},
		{
			name:     "Blame command",
			command:  "git blame file.go",
			expected: true,
		},
		{
			name:     "Diff command with args",
			command:  "git diff HEAD~1 HEAD",
			expected: true,
		},
		{
			name:     "Show command",
			command:  "git show abc123",
			expected: true,
		},
		{
			name:     "ls-files command",
			command:  "git ls-files",
			expected: true,
		},
		{
			name:     "ls-remote command",
			command:  "git ls-remote origin",
			expected: true,
		},
		{
			name:     "grep command",
			command:  "git grep 'pattern' -- '*.go'",
			expected: true,
		},
		{
			name:     "shortlog command",
			command:  "git shortlog -sn",
			expected: true,
		},
		{
			name:     "describe command",
			command:  "git describe --tags",
			expected: true,
		},
		{
			name:     "rev-parse command",
			command:  "git rev-parse HEAD",
			expected: true,
		},
		{
			name:     "cat-file command",
			command:  "git cat-file -p HEAD",
			expected: true,
		},
		{
			name:     "help command",
			command:  "git help commit",
			expected: true,
		},
		{
			name:     "whatchanged command",
			command:  "git whatchanged",
			expected: false,
		},
		{
			name:     "reflog command",
			command:  "git reflog",
			expected: true,
		},
		{
			name:     "name-rev command",
			command:  "git name-rev HEAD",
			expected: true,
		},

		// Common modifying commands
		{
			name:     "Commit command",
			command:  "git commit -m 'message'",
			expected: false,
		},
		{
			name:     "Add command",
			command:  "git add file.go",
			expected: false,
		},
		{
			name:     "Pull command",
			command:  "git pull origin main",
			expected: false,
		},
		{
			name:     "Push command",
			command:  "git push origin main",
			expected: false,
		},
		{
			name:     "Merge command",
			command:  "git merge feature-branch",
			expected: false,
		},
		{
			name:     "Reset command",
			command:  "git reset --hard HEAD",
			expected: false,
		},
		{
			name:     "Rebase command",
			command:  "git rebase main",
			expected: false,
		},
		{
			name:     "Cherry-pick command",
			command:  "git cherry-pick abc123",
			expected: false,
		},

		// Special case: remote
		{
			name:     "Remote with no args",
			command:  "git remote",
			expected: true,
		},
		{
			name:     "Remote with -v flag",
			command:  "git remote -v",
			expected: true,
		},
		{
			name:     "Remote show",
			command:  "git remote show origin",
			expected: true,
		},
		{
			name:     "Remote get-url",
			command:  "git remote get-url origin",
			expected: true,
		},
		{
			name:     "Remote add (modifying)",
			command:  "git remote add origin https://github.com/user/repo.git",
			expected: false,
		},
		{
			name:     "Remote remove (modifying)",
			command:  "git remote remove origin",
			expected: false,
		},
		{
			name:     "Remote rename (modifying)",
			command:  "git remote rename origin upstream",
			expected: false,
		},
		{
			name:     "Remote set-url (modifying)",
			command:  "git remote set-url origin https://github.com/user/repo.git",
			expected: false,
		},

		// Special case: branch
		{
			name:     "Branch with no args",
			command:  "git branch",
			expected: true,
		},
		{
			name:     "Branch with -l flag",
			command:  "git branch -l",
			expected: true,
		},
		{
			name:     "Branch with -a flag",
			command:  "git branch -a",
			expected: true,
		},
		{
			name:     "Branch with -r flag",
			command:  "git branch -r",
			expected: true,
		},
		{
			name:     "Branch with --list flag",
			command:  "git branch --list",
			expected: true,
		},
		{
			name:     "Branch with --all flag",
			command:  "git branch --all",
			expected: true,
		},
		{
			name:     "Branch with --remotes flag",
			command:  "git branch --remotes",
			expected: true,
		},
		{
			name:     "Branch create (modifying)",
			command:  "git branch feature-branch",
			expected: false,
		},
		{
			name:     "Branch delete (modifying)",
			command:  "git branch -d feature-branch",
			expected: false,
		},
		{
			name:     "Branch force delete (modifying)",
			command:  "git branch -D feature-branch",
			expected: false,
		},
		{
			name:     "Branch move (modifying)",
			command:  "git branch -m old-name new-name",
			expected: false,
		},

		// Special case: tag
		{
			name:     "Tag with no args",
			command:  "git tag",
			expected: true,
		},
		{
			name:     "Tag with -l flag",
			command:  "git tag -l",
			expected: true,
		},
		{
			name:     "Tag with --list flag",
			command:  "git tag --list",
			expected: true,
		},
		{
			name:     "Tag with pattern",
			command:  "git tag -l 'v1.*'",
			expected: true,
		},
		{
			name:     "Tag create (modifying)",
			command:  "git tag v1.0.0",
			expected: false,
		},
		{
			name:     "Tag create with message (modifying)",
			command:  "git tag -a v1.0.0 -m 'Version 1.0.0'",
			expected: false,
		},
		{
			name:     "Tag delete (modifying)",
			command:  "git tag -d v1.0.0",
			expected: false,
		},

		// Special case: config
		{
			name:     "Config get",
			command:  "git config --get user.name",
			expected: true,
		},
		{
			name:     "Config list",
			command:  "git config --list",
			expected: true,
		},
		{
			name:     "Config list short flag",
			command:  "git config -l",
			expected: true,
		},
		{
			name:     "Config get-all",
			command:  "git config --get-all remote.origin.fetch",
			expected: true,
		},
		{
			name:     "Config get-regexp",
			command:  "git config --get-regexp '^user.'",
			expected: true,
		},
		{
			name:     "Config get-urlmatch",
			command:  "git config --get-urlmatch http github.com",
			expected: true,
		},
		{
			name:     "Config set (modifying)",
			command:  "git config user.name 'John Doe'",
			expected: false,
		},
		{
			name:     "Config set global (modifying)",
			command:  "git config --global user.email 'john@example.com'",
			expected: false,
		},
		{
			name:     "Config unset (modifying)",
			command:  "git config --unset user.name",
			expected: false,
		},

		// Edge cases
		{
			name:     "Mixed case command (is not a valid git command)",
			command:  "git StAtUs",
			expected: false,
		},
		{
			name:     "Command with extra spaces",
			command:  "git   status   -s",
			expected: true,
		},
	}

	// Run all test cases
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := githelpers.IsReadOnlyGitCommand(tc.command)
			assert.Equal(t, tc.expected, result, "Command: %s", tc.command)
		})
	}
}

// different variations of the same command type.
func TestIsReadOnlyGitCommandTable(t *testing.T) {
	// Test tables for specific command categories
	t.Run("Standard read-only commands", func(t *testing.T) {
		commands := []string{
			"git status",
			"git log",
			"git blame",
			"git diff",
			"git show",
			"git ls-files",
			"git ls-remote",
			"git grep",
			"git shortlog",
			"git describe",
			"git rev-parse",
			"git cat-file",
			"git help",
			"git reflog",
			"git name-rev",
		}

		for _, cmd := range commands {
			assert.True(t, githelpers.IsReadOnlyGitCommand(cmd), "Expected read-only: %s", cmd)
		}
	})

	t.Run("Standard modifying commands", func(t *testing.T) {
		commands := []string{
			"git commit",
			"git add",
			"git rm",
			"git push",
			"git pull",
			"git fetch",
			"git merge",
			"git rebase",
			"git reset",
			"git stash",
			"git apply",
			"git cherry-pick",
			"git revert",
			"git clone",
			"git init",
		}

		for _, cmd := range commands {
			assert.False(t, githelpers.IsReadOnlyGitCommand(cmd), "Expected modifying: %s", cmd)
		}
	})
}

// TestRemoteCommandVariations tests specifically the different variations of the git remote command.
func TestRemoteCommandVariations(t *testing.T) {
	readOnlyRemoteCommands := []string{
		"git remote",
		"git remote -v",
		"git remote show origin",
		"git remote get-url origin",
	}

	modifyingRemoteCommands := []string{
		"git remote add origin https://github.com/user/repo.git",
		"git remote remove origin",
		"git remote set-url origin https://github.com/user/repo.git",
		"git remote rename origin upstream",
		"git remote prune origin",
		"git remote update",
	}

	for _, cmd := range readOnlyRemoteCommands {
		assert.True(t, githelpers.IsReadOnlyGitCommand(cmd), "Expected read-only: %s", cmd)
	}

	for _, cmd := range modifyingRemoteCommands {
		assert.False(t, githelpers.IsReadOnlyGitCommand(cmd), "Expected modifying: %s", cmd)
	}
}

// TestBranchCommandVariations tests specifically the different variations of the git branch command.
func TestBranchCommandVariations(t *testing.T) {
	readOnlyBranchCommands := []string{
		"git branch",
		"git branch -l",
		"git branch -a",
		"git branch -r",
		"git branch --list",
		"git branch --all",
		"git branch --remotes",
		"git branch -v",
		"git branch -vv",
		"git branch --verbose",
	}

	modifyingBranchCommands := []string{
		"git branch new-feature",
		"git branch -d old-feature",
		"git branch -D old-feature",
		"git branch -m old-name new-name",
		"git branch --move old-name new-name",
		"git branch -c existing-branch new-branch",
		"git branch --copy existing-branch new-branch",
	}

	for _, cmd := range readOnlyBranchCommands {
		assert.True(t, githelpers.IsReadOnlyGitCommand(cmd), "Expected read-only: %s", cmd)
	}

	for _, cmd := range modifyingBranchCommands {
		assert.False(t, githelpers.IsReadOnlyGitCommand(cmd), "Expected modifying: %s", cmd)
	}
}

// TestTagCommandVariations tests specifically the different variations of the git tag command.
func TestTagCommandVariations(t *testing.T) {
	readOnlyTagCommands := []string{
		"git tag",
		"git tag -l",
		"git tag --list",
		"git tag -l 'v1.*'",
		"git tag --list 'v1.*'",
	}

	modifyingTagCommands := []string{
		"git tag v1.0.0",
		"git tag -a v1.0.0 -m 'Release version'",
		"git tag -d v1.0.0",
		"git tag --delete v1.0.0",
		"git tag -f v1.0.0",
		"git tag --force v1.0.0",
	}

	for _, cmd := range readOnlyTagCommands {
		assert.True(t, githelpers.IsReadOnlyGitCommand(cmd), "Expected read-only: %s", cmd)
	}

	for _, cmd := range modifyingTagCommands {
		assert.False(t, githelpers.IsReadOnlyGitCommand(cmd), "Expected modifying: %s", cmd)
	}
}

// TestConfigCommandVariations tests specifically the different variations of the git config command.
func TestConfigCommandVariations(t *testing.T) {
	readOnlyConfigCommands := []string{
		"git config --get user.name",
		"git config --list",
		"git config -l",
		"git config --get-all remote.origin.fetch",
		"git config --get-regexp '^user.'",
		"git config --get-urlmatch http github.com",
	}

	modifyingConfigCommands := []string{
		"git config user.name 'John Doe'",
		"git config --global user.email 'john@example.com'",
		"git config --unset user.name",
		"git config --unset-all user.email",
		"git config --add section.key value",
		"git config --replace-all section.key value",
		"git config --local core.autocrlf true",
	}

	for _, cmd := range readOnlyConfigCommands {
		assert.True(t, githelpers.IsReadOnlyGitCommand(cmd), "Expected read-only: %s", cmd)
	}

	for _, cmd := range modifyingConfigCommands {
		assert.False(t, githelpers.IsReadOnlyGitCommand(cmd), "Expected modifying: %s", cmd)
	}
}

func TestParseGitCommand_Undo(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    *githelpers.GitCommand
		wantErr bool
	}{
		{
			name:    "undo command is valid",
			command: "git undo",
			want: &githelpers.GitCommand{
				Name:       "undo",
				Args:       []string{},
				Supported:  true,
				Type:       githelpers.Custom,
				IsReadOnly: false,
			},
			wantErr: false,
		},
		{
			name:    "undo with --log is read-only",
			command: "git undo --log",
			want: &githelpers.GitCommand{
				Name:       "undo",
				Args:       []string{"--log"},
				Supported:  true,
				Type:       githelpers.Custom,
				IsReadOnly: true,
			},
			wantErr: false,
		},
		{
			name:    "undo with --hook is not supported",
			command: "git undo --hook",
			want: &githelpers.GitCommand{
				Name:       "undo",
				Args:       []string{"--hook"},
				Supported:  false,
				Type:       githelpers.Custom,
				IsReadOnly: false,
			},
			wantErr: false,
		},
		{
			name:    "undo with other args is valid and not read-only",
			command: "git undo --some-arg value",
			want: &githelpers.GitCommand{
				Name:       "undo",
				Args:       []string{"--some-arg", "value"},
				Supported:  true,
				Type:       githelpers.Custom,
				IsReadOnly: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := githelpers.ParseGitCommand(tt.command)
			if tt.wantErr {
				if gotErr == nil {
					t.Errorf("ParseGitCommand() expected error but got none")
				}
				return
			}
			if gotErr != nil {
				t.Errorf("ParseGitCommand() unexpected error: %v", gotErr)
				return
			}
			if got.Name != tt.want.Name {
				t.Errorf("ParseGitCommand() Name = %v, want %v", got.Name, tt.want.Name)
			}
			if !reflect.DeepEqual(got.Args, tt.want.Args) {
				t.Errorf("ParseGitCommand() Args = %v, want %v", got.Args, tt.want.Args)
			}
			if got.Supported != tt.want.Supported {
				t.Errorf("ParseGitCommand() Valid = %v, want %v", got.Supported, tt.want.Supported)
			}
			if got.Type != tt.want.Type {
				t.Errorf("ParseGitCommand() Type = %v, want %v", got.Type, tt.want.Type)
			}
			if got.IsReadOnly != tt.want.IsReadOnly {
				t.Errorf("ParseGitCommand() IsReadOnly = %v, want %v", got.IsReadOnly, tt.want.IsReadOnly)
			}
		})
	}
}
