package main

import (
	"fmt"
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

// runEnv is like run, but lets you inject extra env vars.
func runEnv(t *testing.T, dir string, extraEnv []string, cmd string, args ...string) string {
	t.Helper()
	c := exec.Command(cmd, args...)
	c.Dir = dir
	c.Env = append(os.Environ(), extraEnv...)
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

	fmt.Println(run(t, tmp, "git", "init", "."))
	fmt.Println(run(t, tmp, "git", "commit", "--allow-empty", "-m", "init"))

	// 2) create a branch named "feature"
	fmt.Println(run(t, tmp, "git", "branch", "feature"))

	// sanity check: branch exists
	pre := run(t, tmp, "git", "branch", "--list", "feature")
	if pre == "" {
		t.Fatal("precondition failed: feature branch was not created")
	}
	fmt.Println(pre)

	// 3) simulate the hook log
	runEnv(t, tmp,
		[]string{"GIT_UNDO_INTERNAL_HOOK=1"},
		"git-undo", "--hook=git branch feature",
	)

	// 3) run our undo
	run(t, tmp, "git-undo")

	// 4) assert the branch is gone
	post := run(t, tmp, "git", "branch", "--list", "feature")
	if post != "" {
		t.Errorf("expected feature branch to be deleted, but still exists:\n%s", post)
	}
}
