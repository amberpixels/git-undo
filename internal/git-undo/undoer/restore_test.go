package undoer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRestoreUndoer_GetUndoCommand(t *testing.T) {
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
			name:    "staged restore",
			command: "git restore --staged file.txt",
			setupMock: func(m *MockGitExec) {
				// No mocks needed for staged restore
			},
			expectedCmd:  "git add file.txt",
			expectedDesc: "Re-stage files: file.txt",
			expectError:  false,
		},
		{
			name:          "worktree restore",
			command:       "git restore file.txt",
			setupMock:     func(m *MockGitExec) {},
			expectError:   true,
			errorContains: "cannot undo git restore --worktree",
		},
		{
			name:          "restore with source",
			command:       "git restore --source=HEAD~1 file.txt",
			setupMock:     func(m *MockGitExec) {},
			expectError:   true,
			errorContains: "cannot undo git restore with --source",
		},
		{
			name:          "no files specified",
			command:       "git restore --staged",
			setupMock:     func(m *MockGitExec) {},
			expectError:   true,
			errorContains: "no files found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := new(MockGitExec)
			tt.setupMock(mockGit)

			cmdDetails, err := ParseGitCommand(tt.command)
			assert.NoError(t, err)

			undoer := &RestoreUndoer{
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
			}

			mockGit.AssertExpectations(t)
		})
	}
}