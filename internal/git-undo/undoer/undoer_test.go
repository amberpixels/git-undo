package undoer

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockGitExec is a mock implementation of GitExec for testing
type MockGitExec struct {
	mock.Mock
}

func (m *MockGitExec) GitRun(subCmd string, args ...string) error {
	mockArgs := []interface{}{subCmd}
	for _, arg := range args {
		mockArgs = append(mockArgs, arg)
	}
	return m.Called(mockArgs...).Error(0)
}

func (m *MockGitExec) GitOutput(subCmd string, args ...string) (string, error) {
	mockArgs := []interface{}{subCmd}
	for _, arg := range args {
		mockArgs = append(mockArgs, arg)
	}
	result := m.Called(mockArgs...)
	return result.String(0), result.Error(1)
}

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