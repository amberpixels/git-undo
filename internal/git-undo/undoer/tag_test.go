package undoer

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTagUndoer_GetUndoCommand(t *testing.T) {
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
			name:    "simple tag creation",
			command: "git tag v1.0.0",
			setupMock: func(m *MockGitExec) {
				m.On("GitRun", "rev-parse", "--verify", "refs/tags/v1.0.0").Return(nil)
			},
			expectedCmd:  "git tag -d v1.0.0",
			expectedDesc: "Delete tag 'v1.0.0'",
			expectError:  false,
		},
		{
			name:    "annotated tag creation",
			command: "git tag -a v2.0.0 -m 'Release version 2.0.0'",
			setupMock: func(m *MockGitExec) {
				m.On("GitRun", "rev-parse", "--verify", "refs/tags/v2.0.0").Return(nil)
			},
			expectedCmd:  "git tag -d v2.0.0",
			expectedDesc: "Delete tag 'v2.0.0'",
			expectError:  false,
		},
		{
			name:          "tag deletion command",
			command:       "git tag -d v1.0.0",
			setupMock:     func(m *MockGitExec) {},
			expectError:   true,
			errorContains: "tag deletion",
		},
		{
			name:          "tag doesn't exist",
			command:       "git tag v3.0.0",
			setupMock: func(m *MockGitExec) {
				m.On("GitRun", "rev-parse", "--verify", "refs/tags/v3.0.0").Return(errors.New("tag not found"))
			},
			expectError:   true,
			errorContains: "does not exist",
		},
		{
			name:          "no tag name found",
			command:       "git tag -l",
			setupMock:     func(m *MockGitExec) {},
			expectError:   true,
			errorContains: "no tag name found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := new(MockGitExec)
			tt.setupMock(mockGit)

			cmdDetails, err := ParseGitCommand(tt.command)
			assert.NoError(t, err)

			undoer := &TagUndoer{
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