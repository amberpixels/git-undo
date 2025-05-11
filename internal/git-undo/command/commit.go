package command

import (
	"errors"
	"os/exec"
	"strings"
)

// CommitUndoer handles undoing git commit operations.
type CommitUndoer struct{}

// GetUndoCommand returns the git command that would undo the commit.
func (c *CommitUndoer) GetUndoCommand(verbose bool) (string, error) {
	_ = verbose
	// TODO: verbose

	// Check if we're at the initial commit (no parent)
	isInitialCmd := exec.Command("git", "rev-parse", "HEAD^{commit}")
	if err := isInitialCmd.Run(); err != nil {
		return "", errors.New("this appears to be the initial commit and cannot be undone this way")
	}

	// Check if this is a merge commit
	isMergeCmd := exec.Command("git", "rev-parse", "-q", "--verify", "HEAD^2")
	isMerge := isMergeCmd.Run() == nil

	if isMerge {
		// if verbose {
		// 	fmt.Println("Detected a merge commit. Will use 'git reset --merge ORIG_HEAD'")
		// }
		// For merge commits, use ORIG_HEAD which points to the state before the merge
		return "git reset --merge ORIG_HEAD", nil
	}

	// Get the commit message to check if it was an amended commit
	commitMsg, err := CheckGitOutput("log", "-1", "--pretty=%B")
	if err == nil && strings.Contains(commitMsg, "[amend]") {
		// if verbose {
		// 	fmt.Println("Detected an amended commit. Will use 'git reset --soft HEAD@{1}'")
		// }
		// For amended commits, use the reflog to go back to the state before the amend
		return "git reset --soft HEAD@{1}", nil
	}

	// Check if the commit is tagged
	tagOutput, err := CheckGitOutput("tag", "--points-at", "HEAD")
	if err == nil && tagOutput != "" {
		_ = err
		// If the commit is tagged, show a warning
		// TODO:
		// fmt.Printf("Warning: The commit being undone has the following tags: %s\n", tagOutput)
		// fmt.Println("These tags will now point to the parent commit.")
	}

	// if verbose {
	// 	fmt.Println("Will undo last commit with 'git reset --soft HEAD~1'")
	// }

	return "git reset --soft HEAD~1", nil
}
