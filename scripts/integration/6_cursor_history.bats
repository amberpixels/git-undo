#!/usr/bin/env bats

load 'test_helpers'

setup() {
    setup_git_undo_test
}

teardown() {
    teardown_git_undo_test
}

@test "6__A: Phase 6a - cursor-history: branching behavior after undo + new command" {
    title "Phase 6a: Cursor-History Branching Test"

    # ============================================================================
    # Flow: A → B → C → undo → undo → F → test undo/undo behavior
    # ============================================================================

    # Create sequence A, B, C
    echo "A content" > fileA.txt
    git add fileA.txt  # A

    echo "B content" > fileB.txt
    git add fileB.txt  # B

    echo "C content" > fileC.txt
    git add fileC.txt  # C

    # Now we have: A, B, C staged
    run_verbose git status --porcelain
    assert_success
    assert_output --partial "A  fileA.txt"
    assert_output --partial "A  fileB.txt"
    assert_output --partial "A  fileC.txt"

    # Undo twice: should leave only A staged
    run_verbose git undo  # Undo C
    run_verbose git undo  # Undo B

    run_verbose git status --porcelain
    assert_success
    assert_output --partial "A  fileA.txt"
    assert_output --partial "?? fileB.txt"
    assert_output --partial "?? fileC.txt"

    # Do new action F (branching occurs)
    echo "F content" > fileF.txt
    git add fileF.txt  # F

    # Now we have: A and F staged
    run_verbose git status --porcelain
    assert_success
    assert_output --partial "A  fileA.txt"
    assert_output --partial "A  fileF.txt"

    # Test 1: After branching, we can still undo commands (branch-aware behavior)
    run_verbose git undo  # Undo F, leaving only A staged

    run_verbose git status --porcelain
    assert_success
    assert_output --partial "A  fileA.txt"
    refute_output --partial "A  fileF.txt"

    # With branch-aware logging, we can now undo A after the branch
    run_verbose git undo  # Should now succeed and undo A
    assert_success

    run_verbose git status --porcelain
    assert_success
    refute_output --partial "A  fileA.txt"
    assert_output --partial "?? fileA.txt"

    # Test 2: After branch truncation, undo/redo behavior resets
    # Since we branched from A to F, previous undoed commands (B,C) are no longer accessible
    # So git undo should not have anything to redo
    run git undo
    assert_failure  # Should fail because no undoed commands are available after truncation

    # Test 3: We can still do normal undo operations on the current branch (A, F)
    git add fileA.txt  # Re-add A
    git add fileF.txt  # Re-add F

    # Verify both A and F are staged after re-adding
    run_verbose git status --porcelain
    assert_success
    assert_output --partial "A  fileA.txt"
    assert_output --partial "A  fileF.txt"

    run git undo  # Should undo F
    assert_success

    run git status --porcelain
    assert_success
    assert_output --partial "A  fileA.txt"
    refute_output --partial "A  fileF.txt"

    # Test 4: Use explicit git undo undo to redo F
    run_verbose git undo undo --verbose  # This should redo F (git undo undo behavior)
    assert_success

    run git status --porcelain
    assert_success
    assert_output --partial "A  fileA.txt"
    assert_output --partial "A  fileF.txt"

    print "Phase 6a completed - tested branching behavior with branch-aware logging"
}
