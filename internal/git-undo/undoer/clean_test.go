package undoer_test

import (
	"errors"
	"testing"

	"github.com/amberpixels/git-undo/internal/git-undo/undoer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanUndoer_GetUndoCommand(t *testing.T) {
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
			name:          "dry-run clean",
			command:       "git clean -n",
			setupMock:     func(_ *MockGitExec) {},
			expectError:   true,
			errorContains: "dry-run clean operations don't modify files",
		},
		{
			name:    "clean with force",
			command: "git clean -f",
			setupMock: func(m *MockGitExec) {
				m.On("GitOutput", "ls", "-la", ".git/git-undo/clean-backups").Return("", errors.New("no backups"))
			},
			expectError:   true,
			errorContains: "permanently removes untracked files that cannot be recovered",
		},
		{
			name:    "clean directories",
			command: "git clean -fd",
			setupMock: func(m *MockGitExec) {
				m.On("GitOutput", "ls", "-la", ".git/git-undo/clean-backups").Return("", errors.New("no backups"))
			},
			expectError:   true,
			errorContains: "permanently removes untracked files that cannot be recovered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := new(MockGitExec)
			tt.setupMock(mockGit)

			cmdDetails, err := undoer.ParseGitCommand(tt.command)
			require.NoError(t, err)

			cleanUndoer := undoer.NewCleanUndoerForTest(mockGit, cmdDetails)

			undoCmds, err := cleanUndoer.GetUndoCommands()

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
			}

			mockGit.AssertExpectations(t)
		})
	}
}
