package undoer_test

import (
	"errors"
	"testing"

	"github.com/amberpixels/git-undo/internal/git-undo/undoer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCherryPickUndoer_GetUndoCommand(t *testing.T) {
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
			name:    "committed cherry-pick",
			command: "git cherry-pick abc123",
			setupMock: func(m *MockGitExec) {
				m.On("GitOutput", "rev-parse", "HEAD").Return("def456", nil)
				m.On("GitOutput", "rev-parse", "--verify", "CHERRY_PICK_HEAD").Return("", errors.New("not found"))
				m.On("GitOutput", "reflog", "-1", "--format=%s").Return("cherry-pick: abc123", nil)
				m.On("GitOutput", "rev-parse", "HEAD~1").Return("xyz789", nil)
				m.On("GitOutput", "diff", "--cached", "--name-only").Return("", nil)
				m.On("GitOutput", "diff", "--name-only").Return("", nil)
			},
			expectedCmd:  "git reset --soft xyz789",
			expectedDesc: "Remove cherry-pick commit def456",
			expectError:  false,
		},
		{
			name:         "cherry-pick with no-commit",
			command:      "git cherry-pick --no-commit abc123",
			setupMock:    func(_ *MockGitExec) {},
			expectedCmd:  "git reset --mixed HEAD",
			expectedDesc: "Reset staged cherry-pick changes",
			expectError:  false,
		},
		{
			name:    "ongoing cherry-pick",
			command: "git cherry-pick abc123",
			setupMock: func(m *MockGitExec) {
				m.On("GitOutput", "rev-parse", "HEAD").Return("def456", nil)
				m.On("GitOutput", "rev-parse", "--verify", "CHERRY_PICK_HEAD").Return("abc123", nil)
			},
			expectedCmd:  "git cherry-pick --abort",
			expectedDesc: "Abort ongoing cherry-pick operation",
			expectError:  false,
		},
		{
			name:    "non-cherry-pick commit",
			command: "git cherry-pick abc123",
			setupMock: func(m *MockGitExec) {
				m.On("GitOutput", "rev-parse", "HEAD").Return("def456", nil)
				m.On("GitOutput", "rev-parse", "--verify", "CHERRY_PICK_HEAD").Return("", errors.New("not found"))
				m.On("GitOutput", "reflog", "-1", "--format=%s").Return("commit: regular message", nil)
				m.On("GitOutput", "log", "-1", "--format=%s", "HEAD").Return("Regular commit", nil)
			},
			expectError:   true,
			errorContains: "does not appear to be a cherry-pick commit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := new(MockGitExec)
			tt.setupMock(mockGit)

			cmdDetails, err := undoer.ParseGitCommand(tt.command)
			require.NoError(t, err)

			cherryPickUndoer := undoer.NewCherryPickUndoerForTest(mockGit, cmdDetails)

			undoCmd, err := cherryPickUndoer.GetUndoCommand()

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
