package main

import (
	"os"
	"os/exec"
	"testing"
)

// run wraps exec.Command and fails the test on error.
func run(t *testing.T, dir, cmd string, args ...string) string {
	t.Helper()
	c := exec.Command(cmd, args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("`%s %v` failed: %v\n%s", cmd, args, err, out)
	}
	return string(out)
}

func TestUndoBranch(t *testing.T) {
	// 1) create a temp dir and init a repo
	tmp, err := os.MkdirTemp("", "gitundo-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	run(t, tmp, "git", "init", ".")

	// 2) create a branch named "feature"
	run(t, tmp, "git", "branch", "feature")

	// sanity check: branch exists
	pre := run(t, tmp, "git", "branch", "--list", "feature")
	if pre == "" {
		t.Fatal("precondition failed: feature branch was not created")
	}

	// 3) run our undo
	run(t, tmp, "git-undo")

	// 4) assert the branch is gone
	post := run(t, tmp, "git", "branch", "--list", "feature")
	if post != "" {
		t.Errorf("expected feature branch to be deleted, but still exists:\n%s", post)
	}
}
