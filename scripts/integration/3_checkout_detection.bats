#!/usr/bin/env bats

load 'test_helpers'

setup() {
    setup_git_undo_test
}

teardown() {
    teardown_git_undo_test
}

@test "3__A: git undo checkout/switch detection - warns and suggests git back" {
    title "Phase 3A: Checkout/Switch Detection Test: Testing that git undo warns for checkout/switch commands"

    # Setup: Create initial commit structure for testing
    echo "initial content" > initial.txt
    git add initial.txt
    git commit -m "Initial commit"

    echo "main content" > main.txt
    git add main.txt
    git commit -m "Main content commit"

    # ============================================================================
    # PHASE 3A-1: Test git checkout detection
    # ============================================================================
    title "Phase 3A-1: Testing git checkout detection..."

    # Create a test branch
    git branch test-branch

    # Perform checkout operation (should be tracked)
    git checkout test-branch

    # Verify we're on the test branch
    run git branch --show-current
    assert_success
    assert_output "test-branch"

    # Try git undo - should warn about checkout command
    run_verbose git undo 2>&1
    assert_success
    assert_output --partial "can't be undone"
    assert_output --partial "git back"

    # ============================================================================
    # PHASE 3A-2: Test git switch -c (branch creation) - should show warning
    # ============================================================================
    title "Phase 3A-2: Testing git switch -c warning..."

    # Switch back to main first
    git checkout main

    # Create a new branch using git switch -c
    git switch -c feature-switch

    # Verify we're on the new branch
    run git branch --show-current
    assert_success
    assert_output "feature-switch"

    # Try git undo - should warn that switch can't be undone and suggest git back
    run_verbose git undo 2>&1
    assert_success
    assert_output --partial "can't be undone"
    assert_output --partial "git back"

    # Verify we're still on the feature-switch branch (no actual undo happened)
    run git branch --show-current
    assert_success
    assert_output "feature-switch"

    # ============================================================================
    # PHASE 3A-3: Test regular git switch - should show warning
    # ============================================================================
    title "Phase 3A-3: Testing regular git switch warning..."

    # Add content to feature branch
    echo "feature content" > feature.txt
    git add feature.txt
    git commit -m "Feature content"

    # Switch back to main
    git switch main

    # Verify we're on main
    run git branch --show-current
    assert_success
    assert_output "main"

    # Try git undo - should warn about switch command and suggest git back
    run_verbose git undo 2>&1
    assert_success
    assert_output --partial "can't be undone"
    assert_output --partial "git back"

    # Verify we're still on main (no actual undo happened)
    run git branch --show-current
    assert_success
    assert_output "main"

    # ============================================================================
    # PHASE 3A-4: Test that git back works as expected for switch/checkout operations
    # ============================================================================
    title "Phase 3A-4: Testing that git back works for switch/checkout operations..."

    # Use git back to go back to the previous branch (should be feature-switch)
    run_verbose git back
    assert_success

    # Verify we're back on feature-switch
    run git branch --show-current
    assert_success
    assert_output "feature-switch"

    # ============================================================================
    # PHASE 3A-5: Test mixed commands - ensure warning only appears for switch/checkout
    # ============================================================================
    title "Phase 3A-5: Testing that warning only appears for switch/checkout commands..."

    # Create and stage a file
    echo "test file" > test-file.txt
    git add test-file.txt

    # Try git undo - should work normally (no warning about git back)
    run_verbose git undo
    assert_success
    refute_output --partial "can't be undone"
    refute_output --partial "git back"

    # Verify file was unstaged
    run_verbose git status --porcelain
    assert_success
    assert_output --partial "?? test-file.txt"

    print "Checkout/switch detection integration test completed successfully!"
}
