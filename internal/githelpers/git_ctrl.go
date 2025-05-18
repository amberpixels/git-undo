package githelpers

// GitCtrl stores required for the app git controller operations.
type GitCtrl interface {
	GetCurrentGitRef() (string, error)
	GetRepoGitDir() (string, error)
}

// defaultGitCtrl is the default implementation of GitCtrl
type defaultGitCtrl struct{}

func (g *defaultGitCtrl) GetCurrentGitRef() (string, error) { return GetCurrentRef() }
func (g *defaultGitCtrl) GetRepoGitDir() (string, error) {
	gitPaths, err := GetGitPaths()
	if err != nil {
		return "", err
	}
	return gitPaths.RepoGitDir, nil
}

func NewGitCtrl() GitCtrl {
	return &defaultGitCtrl{}
}
