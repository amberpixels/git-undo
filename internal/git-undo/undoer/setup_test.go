package undoer_test

import (
	"github.com/stretchr/testify/mock"
)

// MockGitExec is a mock implementation of GitExec for testing.
type MockGitExec struct {
	mock.Mock
}

func (m *MockGitExec) GitRun(subCmd string, args ...string) error {
	mockArgs := []any{subCmd}
	for _, arg := range args {
		mockArgs = append(mockArgs, arg)
	}
	return m.Called(mockArgs...).Error(0)
}

func (m *MockGitExec) GitOutput(subCmd string, args ...string) (string, error) {
	mockArgs := []any{subCmd}
	for _, arg := range args {
		mockArgs = append(mockArgs, arg)
	}
	result := m.Called(mockArgs...)
	return result.String(0), result.Error(1)
}
