package testutil

import "github.com/amberpixels/git-undo/internal/githelpers"

// MockGitCtrl implements GitCtrl for testing
type MockGitCtrl struct {
	currentRef string
	repoGitDir string
}

func (m *MockGitCtrl) GetCurrentGitRef() (string, error) {
	return m.currentRef, nil
}

func (m *MockGitCtrl) GetRepoGitDir() (string, error) {
	return m.repoGitDir, nil
}

func (m *MockGitCtrl) SwitchRef(ref string) {
	m.currentRef = ref
	return
}

func NewMockGitCtrl(repoGitDir string) *MockGitCtrl {
	return &MockGitCtrl{
		currentRef: "main",
		repoGitDir: repoGitDir,
	}
}

func SwitchRef(ctrl githelpers.GitCtrl, ref string) {
	ctrl.(*MockGitCtrl).SwitchRef(ref)
}
