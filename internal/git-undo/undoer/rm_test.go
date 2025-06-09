package undoer

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRmUndoer_GetUndoCommand(t *testing.T) {
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
			name:    "cached removal",
			command: "git rm --cached file.txt",
			setupMock: func(m *MockGitExec) {
				// No mocks needed for cached removal
			},
			expectedCmd:  "git add file.txt",
			expectedDesc: "Re-add files to index: file.txt",
			expectError:  false,
		},
		{
			name:    "regular file removal",
			command: "git rm file.txt",
			setupMock: func(m *MockGitExec) {
				m.On("GitRun", "rev-parse", "--verify", "HEAD").Return(nil)
			},
			expectedCmd:  "git restore --source=HEAD --staged --worktree file.txt",
			expectedDesc: "Restore removed files: file.txt",
			expectError:  false,
		},
		{
			name:    "recursive removal",
			command: "git rm -r src/",
			setupMock: func(m *MockGitExec) {
				m.On("GitRun", "rev-parse", "--verify", "HEAD").Return(nil)
			},
			expectedCmd:  "git restore --source=HEAD --staged --worktree src/",
			expectedDesc: "Restore removed files: src/",
			expectError:  false,
		},
		{
			name:    "no HEAD commit",
			command: "git rm file.txt",
			setupMock: func(m *MockGitExec) {
				m.On("GitRun", "rev-parse", "--verify", "HEAD").Return(errors.New("no HEAD"))
			},
			expectError:   true,
			errorContains: "no HEAD commit exists",
		},
		{
			name:          "dry run command",
			command:       "git rm -n file.txt",
			setupMock:     func(m *MockGitExec) {},
			expectError:   true,
			errorContains: "dry-run",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := new(MockGitExec)
			tt.setupMock(mockGit)

			cmdDetails, err := ParseGitCommand(tt.command)
			assert.NoError(t, err)

			undoer := &RmUndoer{
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