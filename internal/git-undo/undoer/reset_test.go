package undoer

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
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
				m.On("GitOutput", "reflog", "-n", "2", "--format=%H %s").Return("abc123 reset: moving to HEAD~1\ndef456 commit: test message", nil)
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
				m.On("GitOutput", "reflog", "-n", "2", "--format=%H %s").Return("abc123 reset: moving to HEAD~1\ndef456 commit: test message", nil)
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
				m.On("GitOutput", "reflog", "-n", "2", "--format=%H %s").Return("abc123 reset: moving to HEAD~1\ndef456 commit: test message", nil)
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

			cmdDetails, err := ParseGitCommand(tt.command)
			assert.NoError(t, err)

			undoer := &ResetUndoer{
				git:         mockGit,
				originalCmd: cmdDetails,
			}

			undoCmd, err := undoer.GetUndoCommand()

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, undoCmd)
				assert.Equal(t, tt.expectedCmd, undoCmd.Command)
				assert.Equal(t, tt.expectedDesc, undoCmd.Description)
				if tt.expectWarnings {
					assert.NotEmpty(t, undoCmd.Warnings)
				}
			}

			mockGit.AssertExpectations(t)
		})
	}
}