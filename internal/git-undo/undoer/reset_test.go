package undoer_test

import (
	"errors"
	"testing"

	"github.com/amberpixels/git-undo/internal/git-undo/undoer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResetUndoer_GetUndoCommand(t *testing.T) {
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
			name:    "soft reset",
			command: "git reset --soft HEAD~1",
			setupMock: func(m *MockGitExec) {
				m.On("GitOutput", "rev-parse", "HEAD").Return("abc123", nil)
				m.On("GitOutput", "reflog", "-n", "2", "--format=%H %s").
					Return("abc123 reset: moving to HEAD~1\ndef456 commit: test message", nil)
			},
			expectedCmd:  "git reset --soft def456",
			expectedDesc: "Reset HEAD back to def456 (preserving index and working tree)",
			expectError:  false,
		},
		{
			name:    "mixed reset (default)",
			command: "git reset HEAD~1",
			setupMock: func(m *MockGitExec) {
				m.On("GitOutput", "rev-parse", "HEAD").Return("abc123", nil)
				m.On("GitOutput", "reflog", "-n", "2", "--format=%H %s").
					Return("abc123 reset: moving to HEAD~1\ndef456 commit: test message", nil)
			},
			expectedCmd:  "git reset def456",
			expectedDesc: "Reset HEAD and index back to def456 (preserving working tree)",
			expectError:  false,
		},
		{
			name:    "hard reset with warnings",
			command: "git reset --hard HEAD~1",
			setupMock: func(m *MockGitExec) {
				m.On("GitOutput", "rev-parse", "HEAD").Return("abc123", nil)
				m.On("GitOutput", "reflog", "-n", "2", "--format=%H %s").
					Return("abc123 reset: moving to HEAD~1\ndef456 commit: test message", nil)
				m.On("GitOutput", "diff", "--cached", "--name-only").Return("staged.txt", nil)
				m.On("GitOutput", "diff", "--name-only").Return("unstaged.txt", nil)
			},
			expectedCmd:    "git reset --hard def456",
			expectedDesc:   "Reset HEAD, index, and working tree back to def456",
			expectError:    false,
			expectWarnings: true,
		},
		{
			name:    "no HEAD available",
			command: "git reset HEAD~1",
			setupMock: func(m *MockGitExec) {
				m.On("GitOutput", "rev-parse", "HEAD").Return("", errors.New("no HEAD"))
			},
			expectError:   true,
			errorContains: "cannot determine current HEAD",
		},
		{
			name:    "insufficient reflog history",
			command: "git reset HEAD~1",
			setupMock: func(m *MockGitExec) {
				m.On("GitOutput", "rev-parse", "HEAD").Return("abc123", nil)
				m.On("GitOutput", "reflog", "-n", "2", "--format=%H %s").Return("abc123 reset: moving to HEAD~1", nil)
			},
			expectError:   true,
			errorContains: "insufficient reflog history",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := new(MockGitExec)
			tt.setupMock(mockGit)

			cmdDetails, err := undoer.ParseGitCommand(tt.command)
			require.NoError(t, err)

			resetUndoer := undoer.NewResetUndoerForTest(mockGit, cmdDetails)

			undoCmds, err := resetUndoer.GetUndoCommands()

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
