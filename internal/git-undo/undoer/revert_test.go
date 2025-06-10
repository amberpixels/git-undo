package undoer_test

import (
	"errors"
	"testing"

	"github.com/amberpixels/git-undo/internal/git-undo/undoer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRevertUndoer_GetUndoCommand(t *testing.T) {
	tests := []struct {
		name          string
		command       string
		setupMock     func(*MockGitExec)
		expectedCmd   string
		expectedDesc  string
		expectError   bool
		errorContains string
	}{
		{
			name:    "committed revert",
			command: "git revert abc123",
			setupMock: func(m *MockGitExec) {
				m.On("GitOutput", "rev-parse", "HEAD").Return("def456", nil)
				m.On("GitOutput", "log", "-1", "--format=%s", "HEAD").Return("Revert \"test commit\"", nil)
				m.On("GitOutput", "rev-parse", "HEAD~1").Return("abc123", nil)
				m.On("GitOutput", "diff", "--cached", "--name-only").Return("", nil)
				m.On("GitOutput", "diff", "--name-only").Return("", nil)
			},
			expectedCmd:  "git reset --soft abc123",
			expectedDesc: "Remove revert commit def456",
			expectError:  false,
		},
		{
			name:         "revert with no-commit",
			command:      "git revert --no-commit abc123",
			setupMock:    func(_ *MockGitExec) {},
			expectedCmd:  "git reset --mixed HEAD",
			expectedDesc: "Reset staged revert changes",
			expectError:  false,
		},
		{
			name:    "non-revert commit",
			command: "git revert abc123",
			setupMock: func(m *MockGitExec) {
				m.On("GitOutput", "rev-parse", "HEAD").Return("def456", nil)
				m.On("GitOutput", "log", "-1", "--format=%s", "HEAD").Return("Regular commit message", nil)
				m.On("GitOutput", "reflog", "-1", "--format=%s").Return("commit: regular message", nil)
			},
			expectError:   true,
			errorContains: "does not appear to be a revert commit",
		},
		{
			name:    "no parent commit",
			command: "git revert abc123",
			setupMock: func(m *MockGitExec) {
				m.On("GitOutput", "rev-parse", "HEAD").Return("def456", nil)
				m.On("GitOutput", "log", "-1", "--format=%s", "HEAD").Return("Revert \"test commit\"", nil)
				m.On("GitOutput", "rev-parse", "HEAD~1").Return("", errors.New("no parent"))
			},
			expectError:   true,
			errorContains: "cannot find parent commit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := new(MockGitExec)
			tt.setupMock(mockGit)

			cmdDetails, err := undoer.ParseGitCommand(tt.command)
			require.NoError(t, err)

			revertUndoer := undoer.NewRevertUndoerForTest(mockGit, cmdDetails)

			undoCmd, err := revertUndoer.GetUndoCommand()

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, undoCmd)
				assert.Equal(t, tt.expectedCmd, undoCmd.Command)
				assert.Equal(t, tt.expectedDesc, undoCmd.Description)
			}

			mockGit.AssertExpectations(t)
		})
	}
}
