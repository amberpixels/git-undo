#!/usr/bin/env bats

load 'test_helpers'

setup() {
    setup_git_undo_test
}

teardown() {
    teardown_git_undo_test
}

@test "2__A: Phase 2A Commands: git reset, revert, cherry-pick undo functionality" {
    title "Phase 2A-1: Testing git reset, revert, cherry-pick undo functionality"

    # Setup: Create initial commit structure for testing
    echo "initial content" > initial.txt
    git add initial.txt
    git commit -m "Initial commit"

    echo "second content" > second.txt
    git add second.txt
    git commit -m "Second commit"

    echo "third content" > third.txt
    git add third.txt
    git commit -m "Third commit"

    # ============================================================================
    # PHASE 2A-2: Test git reset undo
    # ============================================================================
    title "Phase 2A-2: Testing git reset undo..."

    # Get current commit hash for verification
    run_verbose git rev-parse HEAD
    assert_success
    third_commit="$output"

    # Perform a soft reset to previous commit
    run_verbose git reset --soft HEAD~1

    # Verify we're at the second commit with staged changes
    run_verbose git rev-parse HEAD
    assert_success
    second_commit="$output"

    # Verify third.txt is staged
    run_verbose git diff --cached --name-only
    assert_success
    assert_line "third.txt"

    # Undo the reset (should restore HEAD to third_commit)
    run_verbose git-undo
    assert_success

    # Verify we're back at the third commit
    run git rev-parse HEAD
    assert_success
    assert_output "$third_commit"

    # Test mixed reset undo
    run_verbose git reset HEAD~1

    # Verify second commit with unstaged changes
    run git rev-parse HEAD
    assert_success
    assert_output "$second_commit"

    # Debug: Check what's in the log before undo
    run_verbose git-undo --log

    run_verbose git status --porcelain
    assert_success
    assert_output --partial "?? third.txt"

    # Undo the mixed reset
    run_verbose git-undo
    assert_success

    # Verify restoration
    run git rev-parse HEAD
    assert_success
    assert_output "$third_commit"

    # ============================================================================
    # PHASE 2A-3: Test git revert undo
    # ============================================================================
    title "Phase 2A-3: Testing git revert undo..."

    # Create a commit to revert
    echo "revert-me content" > revert-me.txt
    git add revert-me.txt
    git commit -m "Commit to be reverted"

    # Get the commit hash
    run git rev-parse HEAD
    assert_success
    revert_target="$output"

    # Revert the commit
    git revert --no-edit HEAD

    # Verify revert commit was created
    run git log -1 --format="%s"
    assert_success
    assert_output --partial "Revert"

    # Verify file was removed by revert
    [ ! -f revert-me.txt ]

    # Undo the revert
    run_verbose git-undo
    assert_success

    # Debug: Check git status after undo
    run_verbose git status --porcelain
    run_verbose ls -la revert-me.txt || echo "File not found"

    # Verify we're back to the original commit
    run git rev-parse HEAD
    assert_success
    assert_output "$revert_target"

    # Verify file is back
    [ -f revert-me.txt ]

    # ============================================================================
    # PHASE 2A-4: Test git cherry-pick undo
    # ============================================================================
    title "Phase 2A-4: Testing git cherry-pick undo..."

    # Create a feature branch with a commit to cherry-pick
    git checkout -b feature-cherry
    echo "cherry content" > cherry.txt
    git add cherry.txt
    git commit -m "Cherry-pick target commit"

    # Get the commit hash
    run git rev-parse HEAD
    assert_success
    cherry_commit="$output"

    # Go back to main branch
    git checkout main

    # Record main branch state
    run git rev-parse HEAD
    assert_success
    main_before_cherry="$output"

    # Cherry-pick the commit
    git cherry-pick "$cherry_commit"

    # Verify cherry-pick was successful
    [ -f cherry.txt ]
    run git log -1 --format="%s"
    assert_success
    assert_output "Cherry-pick target commit"

    # Undo the cherry-pick
    run_verbose git-undo
    assert_success

    # Verify we're back to the original main state
    run git rev-parse HEAD
    assert_success
    assert_output "$main_before_cherry"

    # Verify cherry-picked file is gone
    [ ! -f cherry.txt ]

    # ============================================================================
    # PHASE 2A-5: Test git clean undo (expected to fail)
    # ============================================================================
    title "Phase 2A-5: Testing git clean undo (should show unsupported error)..."

    # Create untracked files
    echo "untracked1" > untracked1.txt
    echo "untracked2" > untracked2.txt

    # Verify files exist
    [ -f untracked1.txt ]
    [ -f untracked2.txt ]

    # Clean the files
    git clean -f

    # Verify files are gone
    [ ! -f untracked1.txt ]
    [ ! -f untracked2.txt ]

    # Try to undo clean (should fail with clear error message)
    run_verbose git-undo
    assert_failure
    assert_output --partial "permanently removes untracked files that cannot be recovered"

    print "Phase 2A Commands integration test completed successfully!"
}
