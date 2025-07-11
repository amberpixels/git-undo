#!/usr/bin/env bats

load 'test_helpers'

setup() {
    setup_git_undo_test
}

teardown() {
    teardown_git_undo_test
}

@test "1__A: Phase 1A Commands: git rm, mv, tag, restore undo functionality" {
    title "Phase 1A-1: Testing git rm, mv, tag, restore undo functionality"

    run_verbose git status --porcelain
    assert_success

    # Setup: Create some initial commits so we're not trying to undo the initial commit
    echo "initial content" > initial.txt
    git add initial.txt
    git commit -m "Initial commit"

    echo "second content" > second.txt
    git add second.txt
    git commit -m "Second commit"

    # ============================================================================
    # PHASE 1A-2: Test git tag undo
    # ============================================================================
    title "Phase 1A-2: Testing git tag undo..."

    # Create a tag
    git tag v1.0.0

    # Verify tag exists
    run git tag -l v1.0.0
    assert_success
    assert_output "v1.0.0"

    # Undo the tag creation
    run_verbose git-undo
    assert_success

    # Verify tag is deleted
    run git tag -l v1.0.0
    assert_success
    assert_output ""

    # Test annotated tag
    git tag -a v2.0.0 -m "Release version 2.0.0"

    # Verify tag exists
    run git tag -l v2.0.0
    assert_success
    assert_output "v2.0.0"

    # Undo the annotated tag creation
    run_verbose git-undo
    assert_success

    # Verify tag is deleted
    run git tag -l v2.0.0
    assert_success
    assert_output ""

    # ============================================================================
    # PHASE 1A-3: Test git mv undo
    # ============================================================================
    title "Phase 1A-3: Testing git mv undo..."

    # Create a file to move
    echo "content for moving" > moveme.txt
    git add moveme.txt
    git commit -m "Add file to move"

    # Move the file
    git mv moveme.txt moved.txt

    # Verify file was moved
    [ ! -f moveme.txt ]
    [ -f moved.txt ]

    # Undo the move
    run_verbose git-undo
    assert_success

    # Verify file is back to original name
    [ -f moveme.txt ]
    [ ! -f moved.txt ]

    # Test moving multiple files to directory
    mkdir subdir
    echo "file1 content" > file1.txt
    echo "file2 content" > file2.txt
    git add file1.txt file2.txt
    git commit -m "Add files for directory move"

    # Move files to subdirectory
    git mv file1.txt file2.txt subdir/

    # Verify files were moved
    [ ! -f file1.txt ]
    [ ! -f file2.txt ]
    [ -f subdir/file1.txt ]
    [ -f subdir/file2.txt ]

    # Undo the move
    run_verbose git-undo
    assert_success

    # Verify files are back
    [ -f file1.txt ]
    [ -f file2.txt ]
    [ ! -f subdir/file1.txt ]
    [ ! -f subdir/file2.txt ]

    # ============================================================================
    # PHASE 1A-4: Test git rm undo
    # ============================================================================
    title "Phase 1A-4: Testing git rm undo..."

    # Create a file to remove
    echo "content for removal" > removeme.txt
    git add removeme.txt
    git commit -m "Add file to remove"

    # Test cached removal (--cached flag)
    git rm --cached removeme.txt

    # Verify file is unstaged but still exists
    run git ls-files removeme.txt
    assert_success
    assert_output ""
    [ -f removeme.txt ]

    # Undo the cached removal
    run_verbose git-undo
    assert_success

    # Verify file is back in index
    run git ls-files removeme.txt
    assert_success
    assert_output "removeme.txt"

    # Test full removal
    git rm removeme.txt

    # Verify file is removed from both index and working directory
    run git ls-files removeme.txt
    assert_success
    assert_output ""
    [ ! -f removeme.txt ]

    # Undo the removal
    run_verbose git-undo
    assert_success

    # Verify file is restored
    run git ls-files removeme.txt
    assert_success
    assert_output "removeme.txt"
    [ -f removeme.txt ]

    # ============================================================================
    # PHASE 1A-5: Test git restore undo (staged only)
    # ============================================================================
    title "Phase 1A-5: Testing git restore --staged undo..."

    # Create and stage a file
    echo "staged content" > staged.txt
    git add staged.txt

    # Verify file is staged
    run git diff --cached --name-only
    assert_success
    assert_line "staged.txt"

    # Restore (unstage) the file
    git restore --staged staged.txt

    # Verify file is no longer staged
    run git diff --cached --name-only
    assert_success
    assert_output ""

    # Undo the restore (re-stage the file)
    run_verbose git-undo
    assert_success

    # Verify file is staged again
    run git diff --cached --name-only
    assert_success
    assert_line "staged.txt"

    print "Phase 1A Commands integration test completed successfully!"
}
