#!/usr/bin/env bats

load 'test_helpers'

setup() {
    setup_git_undo_test
}

teardown() {
    teardown_git_undo_test
}

@test "4__A: Additional Commands: git stash, merge, reset --hard, restore, branch undo functionality" {
    title "Phase 4A: Testing additional git command undo functionality"

    # Setup: Create initial commit structure for testing
    echo "initial content" > initial.txt
    git add initial.txt
    git commit -m "Initial commit"

    echo "main content" > main.txt
    git add main.txt
    git commit -m "Main content commit"

    # ============================================================================
    # PHASE 4A-1: Test git stash undo
    # ============================================================================
    title "Phase 4A-1: Testing git stash undo..."

    # Create some changes to stash
    echo "changes to stash" >> main.txt
    echo "new unstaged file" > unstaged.txt

    # Stage one change
    git add unstaged.txt

    # Verify we have both staged and unstaged changes
    run_verbose git status --porcelain
    assert_success
    assert_output --partial "A  unstaged.txt"
    assert_output --partial " M main.txt"

    # Stash the changes
    run_verbose git stash push -m "Test stash message"

    # Verify working directory is clean
    run_verbose git status --porcelain
    assert_success
    assert_output ""

    # Verify files are back to original state
    [ ! -f unstaged.txt ]
    run cat main.txt
    assert_success
    assert_output "main content"

    # Undo the stash (should restore the changes)
    run_verbose git-undo
    assert_success

    # Verify changes are restored
    run_verbose git status --porcelain
    assert_success
    assert_output --partial "A  unstaged.txt"
    assert_output --partial " M main.txt"

    # Clean up for next test
    git reset HEAD unstaged.txt
    git checkout -- main.txt
    rm -f unstaged.txt

    # ============================================================================
    # PHASE 4A-2: Test git reset --hard undo
    # ============================================================================
    title "Phase 4A-2: Testing git reset --hard undo..."

    # Create a commit to reset from
    echo "content to be reset" > reset-test.txt
    git add reset-test.txt
    git commit -m "Commit to be reset with --hard"

    # Get current commit hash
    run git rev-parse HEAD
    assert_success
    current_commit="$output"

    # Create some uncommitted changes
    echo "uncommitted changes" >> main.txt
    echo "untracked file" > untracked.txt

    # Perform hard reset (should lose uncommitted changes)
    git reset --hard HEAD~1

    # Verify we're at previous commit and changes are gone
    run git rev-parse HEAD
    assert_success
    refute_output "$current_commit"
    [ ! -f reset-test.txt ]
    [ -f untracked.txt ]

    # Undo the hard reset
    run_verbose git-undo
    assert_success

    # Verify we're back at the original commit
    run git rev-parse HEAD
    assert_success
    assert_output "$current_commit"
    [ -f reset-test.txt ]

    # ============================================================================
    # PHASE 4A-3: Test git merge undo (fast-forward)
    # ============================================================================
    title "Phase 4A-3: Testing git merge undo..."

    # Create a feature branch with commits
    git checkout -b feature-merge
    echo "feature change 1" > feature1.txt
    git add feature1.txt
    git commit -m "Feature commit 1"

    echo "feature change 2" > feature2.txt
    git add feature2.txt
    git commit -m "Feature commit 2"

    # Record feature branch head
    run git rev-parse HEAD
    assert_success
    feature_head="$output"

    # Switch back to main and record state
    git checkout main
    run git rev-parse HEAD
    assert_success
    main_before_merge="$output"

    # Perform fast-forward merge
    git merge feature-merge

    # Verify merge was successful (should be fast-forward)
    run git rev-parse HEAD
    assert_success
    assert_output "$feature_head"
    [ -f feature1.txt ]
    [ -f feature2.txt ]

    # Undo the merge
    run_verbose git-undo
    assert_success

    # Verify we're back to pre-merge state
    run git rev-parse HEAD
    assert_success
    assert_output "$main_before_merge"
    [ ! -f feature1.txt ]
    [ ! -f feature2.txt ]

    # ============================================================================
    # PHASE 4A-4: Test git branch -D undo (should fail with clear error message)
    # ============================================================================
    title "Phase 4A-4: Testing git branch -D undo (should show unsupported error)..."

    # Verify feature branch still exists
    run git branch --list feature-merge
    assert_success
    assert_output --partial "feature-merge"

    # Delete the feature branch (use -D since it's not merged)
    git branch -D feature-merge

    # Verify branch is deleted
    run git branch --list feature-merge
    assert_success
    assert_output ""

    # Try to undo the branch deletion (should fail with clear error message)
    run_verbose git-undo
    assert_failure
    assert_output --partial "git undo not supported for branch deletion"

    print "Phase 4A: Additional commands integration test completed successfully!"
}
