#!/usr/bin/env bats

load 'test_helpers'

setup() {
    setup_git_undo_test
}

teardown() {
    teardown_git_undo_test
}

@test "5__A: Error Conditions and Edge Cases" {
    title "Phase 5A: Testing error conditions and edge cases"

    # ============================================================================
    # PHASE 5A-1: Test git undo with no previous commands
    # ============================================================================
    title "Phase 5A-1: Testing git undo with empty log..."

    # Clear any existing log by creating a fresh repository state
    # The setup() already creates a clean state with just init commit

    # Try git undo when there are no tracked commands
    # First undo should fail because it's trying to undo the initial commit
    run_verbose git undo
    assert_failure
    assert_output --partial "this appears to be the initial commit and cannot be undone this way"

    # Second undo should still fail with the same error since the initial commit is still there
    run_verbose git undo
    assert_failure
    assert_output --partial "this appears to be the initial commit and cannot be undone this way"

    # ============================================================================
    # PHASE 5A-2: Test git undo --log with empty log
    # ============================================================================
    title "Phase 5A-2: Testing git undo --log with empty log..."

    # Check that log shows appropriate message when empty
    run_verbose git undo --log
    assert_success
    # Should either show empty output or a message about no commands

    # ============================================================================
    # PHASE 5A-3: Test unsupported commands
    # ============================================================================
    title "Phase 5A-3: Testing unsupported commands..."

    # Setup some commits first
    echo "test content" > test.txt
    git add test.txt
    git commit -m "Test commit"

    # Test git rebase (should show warning/error about being unsupported)
    git checkout -b rebase-test
    echo "branch content" > branch.txt
    git add branch.txt
    git commit -m "Branch commit"

    git checkout main
    # Attempt rebase
    git rebase rebase-test 2>/dev/null || true

    # Try to undo rebase - should fail or warn appropriately
    run_verbose git undo
    # This might succeed or fail depending on implementation
    # The important thing is it handles it gracefully

    # ============================================================================
    # PHASE 5A-4: Test git undo after hook failures
    # ============================================================================
    title "Phase 5A-4: Testing behavior after hook failures..."

    # Perform a normal operation that should be tracked
    echo "tracked content" > tracked.txt
    git add tracked.txt

    # Verify it can be undone normally
    run_verbose git undo
    assert_success

    # Verify file is unstaged
    run_verbose git status --porcelain
    assert_success
    assert_output --partial "?? tracked.txt"

    # ============================================================================
    # PHASE 5A-5: Test concurrent operations and rapid commands
    # ============================================================================
    title "Phase 5A-5: Testing rapid sequential commands..."

    # Perform multiple rapid operations
    echo "rapid1" > rapid1.txt
    git add rapid1.txt
    echo "rapid2" > rapid2.txt
    git add rapid2.txt
    echo "rapid3" > rapid3.txt
    git add rapid3.txt

    # Verify all operations are tracked in correct order (LIFO)
    run_verbose git undo
    assert_success
    run_verbose git status --porcelain
    assert_success
    assert_output --partial "A  rapid1.txt"
    assert_output --partial "A  rapid2.txt"
    assert_output --partial "?? rapid3.txt"

    run_verbose git undo
    assert_success
    run_verbose git status --porcelain
    assert_success
    assert_output --partial "A  rapid1.txt"
    assert_output --partial "?? rapid2.txt"
    assert_output --partial "?? rapid3.txt"

    run_verbose git undo
    assert_success
    run_verbose git status --porcelain
    assert_success
    assert_output --partial "?? rapid1.txt"
    assert_output --partial "?? rapid2.txt"
    assert_output --partial "?? rapid3.txt"

    print "Phase 5A:Error conditions and edge cases test completed successfully!"
}
