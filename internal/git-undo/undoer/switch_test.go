package undoer_test

import (
	"errors"
	"testing"

	"github.com/amberpixels/git-undo/internal/git-undo/undoer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSwitchUndoer_GetUndoCommand(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		setupMock      func(*MockGitExec)
		expectedCmd    string
		expectedDesc   string
		expectError    bool
		errorContains  string
		expectWarnings bool
	}{
		{
			name:         "branch creation with -c",
			command:      "git switch -c feature-branch",
			setupMock:    func(_ *MockGitExec) {},
			expectedCmd:  "git branch -D feature-branch",
			expectedDesc: "Delete branch 'feature-branch' created by switch -c",
			expectError:  false,
		},
		{
			name:         "branch creation with --create",
			command:      "git switch --create new-feature",
			setupMock:    func(_ *MockGitExec) {},
			expectedCmd:  "git branch -D new-feature",
			expectedDesc: "Delete branch 'new-feature' created by switch -c",
			expectError:  false,
		},
		{
			name:           "force branch creation with -C",
			command:        "git switch -C hotfix main",
			setupMock:      func(_ *MockGitExec) {},
			expectedCmd:    "git branch -D hotfix",
			expectedDesc:   "Delete branch 'hotfix' created by switch -C",
			expectError:    false,
			expectWarnings: true,
		},
		{
			name:           "force branch creation with --force-create",
			command:        "git switch --force-create existing-branch",
			setupMock:      func(_ *MockGitExec) {},
			expectedCmd:    "git branch -D existing-branch",
			expectedDesc:   "Delete branch 'existing-branch' created by switch -C",
			expectError:    false,
			expectWarnings: true,
		},
		{
			name:    "regular branch switch",
			command: "git switch main",
			setupMock: func(m *MockGitExec) {
				m.On("GitOutput", "rev-parse", "--symbolic-full-name", "@{-1}").Return("refs/heads/feature", nil)
				m.On("GitOutput", "diff", "--cached", "--name-only").Return("", nil)
				m.On("GitOutput", "diff", "--name-only").Return("", nil)
				m.On("GitOutput", "ls-files", "--others", "--exclude-standard").Return("", nil)
			},
			expectedCmd:  "git switch -",
			expectedDesc: "Switch back to previous branch (feature)",
			expectError:  false,
		},
		{
			name:    "branch switch with warnings",
			command: "git switch develop",
			setupMock: func(m *MockGitExec) {
				m.On("GitOutput", "rev-parse", "--symbolic-full-name", "@{-1}").Return("refs/heads/main", nil)
				m.On("GitOutput", "diff", "--cached", "--name-only").Return("staged.txt", nil)
				m.On("GitOutput", "diff", "--name-only").Return("modified.txt", nil)
				m.On("GitOutput", "ls-files", "--others", "--exclude-standard").Return("untracked.txt", nil)
			},
			expectedCmd:    "git switch -",
			expectedDesc:   "Switch back to previous branch (main)",
			expectError:    false,
			expectWarnings: true,
		},
		{
			name:    "no previous branch",
			command: "git switch feature",
			setupMock: func(m *MockGitExec) {
				m.On("GitOutput", "rev-parse", "--symbolic-full-name", "@{-1}").
					Return("", errors.New("no previous branch"))
			},
			expectError:   true,
			errorContains: "no previous branch to return to",
		},
		{
			name:    "empty previous branch",
			command: "git switch feature",
			setupMock: func(m *MockGitExec) {
				m.On("GitOutput", "rev-parse", "--symbolic-full-name", "@{-1}").Return("", nil)
			},
			expectError:   true,
			errorContains: "no previous branch to return to",
		},
		{
			name:    "switch to commit hash",
			command: "git switch abc123",
			setupMock: func(m *MockGitExec) {
				m.On("GitOutput", "rev-parse", "--symbolic-full-name", "@{-1}").Return("refs/heads/main", nil)
				m.On("GitOutput", "diff", "--cached", "--name-only").Return("", nil)
				m.On("GitOutput", "diff", "--name-only").Return("", nil)
				m.On("GitOutput", "ls-files", "--others", "--exclude-standard").Return("", nil)
			},
			expectedCmd:  "git switch -",
			expectedDesc: "Switch back to previous branch (main)",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := new(MockGitExec)
			tt.setupMock(mockGit)

			cmdDetails, err := undoer.ParseGitCommand(tt.command)
			require.NoError(t, err)

			switchUndoer := undoer.NewSwitchUndoerForTest(mockGit, cmdDetails)

			undoCmds, err := switchUndoer.GetUndoCommands()

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				require.Len(t, undoCmds, 1)
				assert.Equal(t, tt.expectedCmd, undoCmds[0].Command)
				assert.Equal(t, tt.expectedDesc, undoCmds[0].Description)
				if tt.expectWarnings {
					assert.NotEmpty(t, undoCmds[0].Warnings)
				}
			}

			mockGit.AssertExpectations(t)
		})
	}
}
