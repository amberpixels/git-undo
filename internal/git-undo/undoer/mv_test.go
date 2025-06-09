package undoer

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMvUndoer_GetUndoCommand(t *testing.T) {
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
			name:    "simple file move",
			command: "git mv old.txt new.txt",
			setupMock: func(m *MockGitExec) {
				m.On("GitRun", "ls-files", "--error-unmatch", "new.txt").Return(nil)
			},
			expectedCmd:  "git mv new.txt old.txt",
			expectedDesc: "Move 'new.txt' back to 'old.txt'",
			expectError:  false,
		},
		{
			name:    "multiple files to directory",
			command: "git mv file1.txt file2.txt src/",
			setupMock: func(m *MockGitExec) {
				m.On("GitRun", "ls-files", "--error-unmatch", "src/file1.txt").Return(nil)
				m.On("GitRun", "ls-files", "--error-unmatch", "src/file2.txt").Return(nil)
			},
			expectedCmd:  "git mv src/file1.txt file1.txt && git mv src/file2.txt file2.txt",
			expectedDesc: "Move files back: 'src/file1.txt' → 'file1.txt', 'src/file2.txt' → 'file2.txt'",
			expectError:  false,
		},
		{
			name:          "insufficient arguments",
			command:       "git mv file1.txt",
			setupMock:     func(m *MockGitExec) {},
			expectError:   true,
			errorContains: "insufficient arguments",
		},
		{
			name:    "destination doesn't exist",
			command: "git mv old.txt new.txt",
			setupMock: func(m *MockGitExec) {
				m.On("GitRun", "ls-files", "--error-unmatch", "new.txt").Return(errors.New("file not found"))
			},
			expectError:   true,
			errorContains: "does not exist in git index",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := new(MockGitExec)
			tt.setupMock(mockGit)

			cmdDetails, err := ParseGitCommand(tt.command)
			assert.NoError(t, err)

			undoer := &MvUndoer{
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