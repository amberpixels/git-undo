package undoer_test

import (
	"testing"

	"github.com/amberpixels/git-undo/internal/git-undo/undoer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMvUndoer_GetUndoCommands(t *testing.T) {
	tests := []struct {
		name          string
		command       string
		setupMock     func(*MockGitExec)
		expectedCmds  []string
		expectedDescs []string
		expectError   bool
		errorContains string
	}{
		{
			name:    "simple file move",
			command: "git mv old.txt new.txt",
			setupMock: func(m *MockGitExec) {
				m.On("GitRun", "ls-files", "--error-unmatch", "new.txt").Return(nil)
			},
			expectedCmds:  []string{"git mv new.txt old.txt"},
			expectedDescs: []string{"Move 'new.txt' back to 'old.txt'"},
			expectError:   false,
		},
		{
			name:    "multiple files to directory",
			command: "git mv file1.txt file2.txt src/",
			setupMock: func(m *MockGitExec) {
				m.On("GitRun", "ls-files", "--error-unmatch", "src/file1.txt").Return(nil)
				m.On("GitRun", "ls-files", "--error-unmatch", "src/file2.txt").Return(nil)
			},
			expectedCmds: []string{
				"git mv src/file1.txt file1.txt",
				"git mv src/file2.txt file2.txt",
			},
			expectedDescs: []string{
				"Move 'src/file1.txt' back to 'file1.txt'",
				"Move 'src/file2.txt' back to 'file2.txt'",
			},
			expectError: false,
		},
		{
			name:          "insufficient arguments",
			command:       "git mv file1.txt",
			setupMock:     func(_ *MockGitExec) {},
			expectError:   true,
			errorContains: "insufficient arguments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := new(MockGitExec)
			tt.setupMock(mockGit)

			cmdDetails, err := undoer.ParseGitCommand(tt.command)
			require.NoError(t, err)

			mvUndoer := undoer.NewMvUndoerForTest(mockGit, cmdDetails)

			undoCmds, err := mvUndoer.GetUndoCommands()

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				require.Len(t, undoCmds, len(tt.expectedCmds))
				for i, cmd := range undoCmds {
					assert.Equal(t, tt.expectedCmds[i], cmd.Command)
					assert.Equal(t, tt.expectedDescs[i], cmd.Description)
				}
			}

			mockGit.AssertExpectations(t)
		})
	}
}
