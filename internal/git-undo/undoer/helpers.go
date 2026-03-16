package undoer

import "strings"

func getShortHash(hash string) string {
	const lenShortHash = 8
	if len(hash) > lenShortHash {
		return hash[:lenShortHash]
	}
	return hash
}

// collectWorkingDirWarnings checks for staged, unstaged, and untracked changes
// and returns appropriate warning messages.
func collectWorkingDirWarnings(git GitExec, conflictContext string, stashHint string) []string {
	var warnings []string

	if out, err := git.GitOutput("diff", "--cached", "--name-only"); err == nil && strings.TrimSpace(out) != "" {
		warnings = append(warnings, "You have staged changes that may conflict with "+conflictContext)
	}

	if out, err := git.GitOutput("diff", "--name-only"); err == nil && strings.TrimSpace(out) != "" {
		warnings = append(warnings, "You have unstaged changes that may conflict with "+conflictContext)
	}

	if out, err := git.GitOutput("ls-files", "--others", "--exclude-standard"); err == nil && strings.TrimSpace(out) != "" {
		warnings = append(warnings, "You have untracked files (these usually don't conflict)")
	}

	if len(warnings) > 0 {
		warnings = append(warnings, "If "+stashHint+" fails, try: 'git stash' first, then undo, then 'git stash pop'")
		warnings = append(warnings, "Or commit your changes first with 'git commit -m \"WIP\"'")
	}

	return warnings
}
