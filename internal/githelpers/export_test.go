package githelpers

// IsReadOnlyGitCommand checks if a git command is read-only based on its name and arguments.
// Note: it's a legacy thing as we already have tests on it.
// Actually we should just do IsReadOnlyCommand = isReadOnlyCommand and test on it.
func IsReadOnlyGitCommand(command string) bool {
	parsed, err := ParseGitCommand(command)
	return err == nil && parsed.Supported && parsed.IsReadOnly
}
