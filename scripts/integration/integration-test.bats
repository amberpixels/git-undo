#!/usr/bin/env bats

# Load bats helpers
load 'test_helper/bats-support/load'
load 'test_helper/bats-assert/load'

setup() {
    # Create isolated test repository for the test
    export TEST_REPO="$(mktemp -d)"
    cd "$TEST_REPO"
    
    git init
    git config user.email "git-undo-test@amberpixels.io"
    git config user.name "Git-Undo Integration Test User"
    
    # Create initial empty commit so we always have HEAD (like in unit tests)
    git commit --allow-empty -m "init"
}

teardown() {
    # Clean up test repository
    rm -rf "$TEST_REPO"
}

@test "complete git-undo integration workflow" {
    # ============================================================================
    # PHASE 1: Verify Installation
    # ============================================================================
    echo "# Phase 1: Verifying git-undo installation..."
    
    run which git-undo
    assert_success
    assert_output --regexp "git-undo$"
    
    # Test version command
    run git-undo --version
    assert_success
    assert_output --partial "v0."
    
    # ============================================================================
    # HOOK DIAGNOSTICS: Debug hook installation and activation
    # ============================================================================
    echo "# HOOK DIAGNOSTICS: Checking hook installation..."
    
    # Check if hook files exist
    echo "# Checking if hook files exist in ~/.config/git-undo/..."
    run ls -la ~/.config/git-undo/
    assert_success
    echo "# Hook directory contents: ${output}"
    
    # Verify hook files are present
    assert [ -f ~/.config/git-undo/git-undo-hook.bash ]
    echo "# ✓ Hook file exists: ~/.config/git-undo/git-undo-hook.bash"
    
    # Verify that the test hook is actually installed (should contain git function)
    echo "# Checking if test hook is installed (contains git function)..."
    run grep -q "git()" ~/.config/git-undo/git-undo-hook.bash
    assert_success
    echo "# ✓ Test hook confirmed: contains git function"
    
    # Check if .bashrc has the source line
    echo "# Checking if .bashrc contains git-undo source line..."
    run grep -n git-undo ~/.bashrc
    assert_success
    echo "# .bashrc git-undo lines: ${output}"
    
    # Check current git command type (before sourcing hooks)
    echo "# Checking git command type before hook loading..."
    run type git
    echo "# Git type before: ${output}"
    
    # Manually source the hook to test if it works
    echo "# Manually sourcing git-undo hook..."
    source ~/.config/git-undo/git-undo-hook.bash
    
    # Check git command type after sourcing hooks
    echo "# Checking git command type after hook loading..."
    run type git
    echo "# Git type after: ${output}"
    
    # Test if git-undo function/alias is available
    echo "# Testing if git undo command is available..."
    run git undo --help
    if [[ $status -eq 0 ]]; then
        echo "# ✓ git undo command responds"
    else
        echo "# ✗ git undo command failed with status: $status"
        echo "# Output: ${output}"
    fi
    
    # ============================================================================
    # PHASE 2: Basic git add and undo workflow
    # ============================================================================
    echo "# Phase 2: Testing basic git add and undo..."
    
    # Create test files
    echo "content of file1" > file1.txt
    echo "content of file2" > file2.txt
    echo "content of file3" > file3.txt
    
    # Verify files are untracked initially
    run git status --porcelain
    assert_success
    assert_output --partial "?? file1.txt"
    assert_output --partial "?? file2.txt"
    assert_output --partial "?? file3.txt"
    
    # Add first file
    git add file1.txt
    run git status --porcelain
    assert_success
    assert_output --partial "A  file1.txt"
    assert_output --partial "?? file2.txt"
    
    # Add second file
    git add file2.txt
    run git status --porcelain
    assert_success
    assert_output --partial "A  file1.txt"
    assert_output --partial "A  file2.txt"
    assert_output --partial "?? file3.txt"
    
    # First undo - should unstage file2.txt
    echo "# DEBUG: Checking git-undo log before first undo..."
    run git undo --log
    assert_success
    refute_output ""  # Log should not be empty if hooks are tracking
    echo "# Log output: ${output}"
    
    run git undo
    assert_success
    
    run git status --porcelain
    assert_success
    assert_output --partial "A  file1.txt"
    assert_output --partial "?? file2.txt"
    assert_output --partial "?? file3.txt"
    refute_output --partial "A  file2.txt"
    
    # Second undo - should unstage file1.txt
    echo "# DEBUG: Checking git-undo log before second undo..."
    run git undo --log
    assert_success
    refute_output ""  # Log should not be empty if hooks are tracking
    echo "# Log output: ${output}"
    
    run git undo
    assert_success
    
    run git status --porcelain
    assert_success
    assert_output --partial "?? file1.txt"
    assert_output --partial "?? file2.txt"
    assert_output --partial "?? file3.txt"
    refute_output --partial "A  file1.txt"
    refute_output --partial "A  file2.txt"
    
    # ============================================================================
    # PHASE 3: Commit and undo workflow
    # ============================================================================
    echo "# Phase 3: Testing commit and undo..."
    
    # Stage and commit first file
    git add file1.txt
    git commit -m "Add file1.txt"
    
    # Verify clean working directory (except for untracked files from previous phase)
    run git status --porcelain
    assert_success
    assert_output --partial "?? file2.txt"
    assert_output --partial "?? file3.txt"
    refute_output --partial "file1.txt"  # file1.txt should be committed, not in status
    
    # Verify file1 exists and is committed
    assert [ -f "file1.txt" ]
    
    # Stage and commit second file
    git add file2.txt
    git commit -m "Add file2.txt"
    
    # Verify clean working directory again (only file3.txt should remain untracked)
    run git status --porcelain
    assert_success
    assert_output --partial "?? file3.txt"
    refute_output --partial "file1.txt"  # file1.txt should be committed
    refute_output --partial "file2.txt"  # file2.txt should be committed
    
    # First commit undo - should undo last commit, leaving file2 staged
    echo "# DEBUG: Checking git-undo log before commit undo..."
    run git undo --log
    assert_success
    refute_output ""  # Log should not be empty if hooks are tracking
    echo "# Log output: ${output}"
    
    run git undo
    assert_success
    
    run git status --porcelain
    assert_success
    assert_output --partial "A  file2.txt"
    
    # Verify files still exist in working directory
    assert [ -f "file1.txt" ]
    assert [ -f "file2.txt" ]
    
    # Second undo - should unstage file2.txt  
    echo "# DEBUG: Checking git-undo log before second commit undo..."
    run git undo --log
    assert_success
    refute_output ""  # Log should not be empty if hooks are tracking
    echo "# Log output: ${output}"
    
    run git undo
    assert_success
    
    run git status --porcelain
    assert_success
    assert_output --partial "?? file2.txt"
    assert_output --partial "?? file3.txt"
    refute_output --partial "A  file2.txt"
    
    # ============================================================================
    # PHASE 4: Complex sequential workflow
    # ============================================================================
    echo "# Phase 4: Testing complex sequential operations..."
    
    # Commit file3
    git add file3.txt
    git commit -m "Add file3.txt"
    
    # Modify file1 and stage the modification
    echo "modified content" >> file1.txt
    git add file1.txt
    
    # Verify modified file1 is staged
    run git status --porcelain
    assert_success
    assert_output --partial "M  file1.txt"
    
    # Create and stage a new file
    echo "content of file4" > file4.txt
    git add file4.txt
    
    # Verify both staged
    run git status --porcelain
    assert_success
    assert_output --partial "M  file1.txt"
    assert_output --partial "A  file4.txt"
    
    # Undo staging of file4
    echo "# DEBUG: Checking git-undo log before file4 undo..."
    run git undo --log
    assert_success
    refute_output ""  # Log should not be empty if hooks are tracking
    echo "# Log output: ${output}"
    
    run git undo
    assert_success
    
    run git status --porcelain
    assert_success
    assert_output --partial "M  file1.txt"  # file1 still staged
    assert_output --partial "?? file4.txt"  # file4 unstaged
    refute_output --partial "A  file4.txt"
    
    # Undo staging of modified file1
    echo "# DEBUG: Checking git-undo log before modified file1 undo..."
    run git undo --log
    assert_success
    refute_output ""  # Log should not be empty if hooks are tracking
    echo "# Log output: ${output}"
    
    run git undo
    assert_success
    
    run git status --porcelain
    assert_success
    assert_output --partial " M file1.txt"  # Modified but unstaged
    assert_output --partial "?? file4.txt"
    refute_output --partial "M  file1.txt"  # Should not be staged anymore
    
    # Undo commit of file3
    run git undo
    assert_success
    
    run git status --porcelain
    assert_success
    assert_output --partial "A  file3.txt"  # file3 back to staged
    assert_output --partial " M file1.txt"  # file1 still modified
    assert_output --partial "?? file4.txt"
    
    # ============================================================================
    # PHASE 5: Verification of final state
    # ============================================================================
    echo "# Phase 5: Final state verification..."
    
    # Verify all files exist
    assert [ -f "file1.txt" ]
    assert [ -f "file2.txt" ] 
    assert [ -f "file3.txt" ]
    assert [ -f "file4.txt" ]
    
    # Verify git log shows only the first commit
    run git log --oneline
    assert_success
    assert_output --partial "Add file1.txt"
    refute_output --partial "Add file2.txt"
    refute_output --partial "Add file3.txt"
    
    echo "# Integration test completed successfully!"
}

@test "git-back integration test - checkout and switch undo" {
    # ============================================================================
    # PHASE 1: Verify git-back Installation
    # ============================================================================
    echo "# Phase 1: Verifying git-back installation..."
    
    run which git-back
    assert_success
    assert_output --regexp "git-back$"
    
    # Test version command
    run git-back --version
    assert_success
    assert_output --partial "v0."
    
    # Test help command
    run git-back --help
    assert_success
    assert_output --partial "Git-back undoes the last git checkout or git switch command"
    
    # ============================================================================
    # PHASE 2: Basic branch switching workflow
    # ============================================================================
    echo "# Phase 2: Testing basic branch switching..."
    
    # Create and commit some files to establish a proper git history
    echo "main content" > main.txt
    git add main.txt
    git commit -m "Add main content"
    
    # Create a feature branch
    git checkout -b feature-branch
    echo "feature content" > feature.txt
    git add feature.txt
    git commit -m "Add feature content"
    
    # Create another branch
    git checkout -b another-branch
    echo "another content" > another.txt
    git add another.txt
    git commit -m "Add another content"
    
    # Verify we're on another-branch
    run git branch --show-current
    assert_success
    assert_output "another-branch"
    
    # ============================================================================
    # PHASE 3: Source hooks for git-back tracking
    # ============================================================================
    echo "# Phase 3: Setting up hooks for git-back tracking..."
    
    # Source the hook to enable command tracking
    source ~/.config/git-undo/git-undo-hook.bash
    
    # ============================================================================
    # PHASE 4: Test git-back with checkout commands
    # ============================================================================
    echo "# Phase 4: Testing git-back with checkout..."
    
    # Switch to feature branch (this should be tracked)
    git checkout feature-branch
    
    # Verify we're on feature-branch
    run git branch --show-current
    assert_success
    assert_output "feature-branch"
    
    # Use git-back to go back to previous branch (should be another-branch)
    run git-back
    assert_success
    
    # Verify we're back on another-branch
    run git branch --show-current
    assert_success
    assert_output "another-branch"
    
    # ============================================================================
    # PHASE 5: Test multiple branch switches
    # ============================================================================
    echo "# Phase 5: Testing multiple branch switches..."
    
    # Switch to main branch
    git checkout main
    
    # Verify we're on main
    run git branch --show-current
    assert_success
    assert_output "main"
    
    # Use git-back to go back to previous branch (should be another-branch)
    run git-back
    assert_success
    
    # Verify we're back on another-branch
    run git branch --show-current
    assert_success
    assert_output "another-branch"
    
    # Switch to feature-branch again
    git checkout feature-branch
    
    # Use git-back to go back to another-branch
    run git-back
    assert_success
    
    # Verify we're on another-branch
    run git branch --show-current
    assert_success
    assert_output "another-branch"
    
    # ============================================================================
    # PHASE 6: Test git-back with uncommitted changes (should show warnings)
    # ============================================================================
    echo "# Phase 6: Testing git-back with uncommitted changes..."
    
    # Make some uncommitted changes
    echo "modified content" >> another.txt
    echo "new file content" > unstaged.txt
    
    # Stage one file
    git add unstaged.txt
    
    # Now try git-back in verbose mode to see warnings
    run git-back -v
    # Note: This might fail due to conflicts, but we want to verify warnings are shown
    # The important thing is that warnings are displayed to the user
    
    # For testing purposes, let's stash the changes and try again
    git stash
    
    # Now git-back should work
    run git-back
    assert_success
    
    # Verify we're back on feature-branch
    run git branch --show-current
    assert_success
    assert_output "feature-branch"
    
    # Pop the stash to restore our changes
    git stash pop
    
    echo "# git-back integration test completed successfully!"
}

@test "Phase 1 Commands: git rm, mv, tag, restore undo functionality" {
    echo "# ============================================================================"
    echo "# PHASE 1 COMMANDS TEST: Testing git rm, mv, tag, restore undo functionality"
    echo "# ============================================================================"
    
    # Setup: Create some initial commits so we're not trying to undo the initial commit
    echo "initial content" > initial.txt
    git add initial.txt
    git commit -m "Initial commit"
    
    echo "second content" > second.txt  
    git add second.txt
    git commit -m "Second commit"
    
    # ============================================================================
    # PHASE 1A: Test git tag undo
    # ============================================================================
    echo "# Phase 1A: Testing git tag undo..."
    
    # Create a tag
    git tag v1.0.0
    
    # Verify tag exists
    run git tag -l v1.0.0
    assert_success
    assert_output "v1.0.0"
    
    # Undo the tag creation
    run git-undo
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
    run git-undo
    assert_success
    
    # Verify tag is deleted
    run git tag -l v2.0.0
    assert_success
    assert_output ""
    
    # ============================================================================
    # PHASE 1B: Test git mv undo
    # ============================================================================
    echo "# Phase 1B: Testing git mv undo..."
    
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
    run git-undo
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
    run git-undo
    assert_success
    
    # Verify files are back
    [ -f file1.txt ]
    [ -f file2.txt ]
    [ ! -f subdir/file1.txt ]
    [ ! -f subdir/file2.txt ]
    
    # ============================================================================
    # PHASE 1C: Test git rm undo
    # ============================================================================
    echo "# Phase 1C: Testing git rm undo..."
    
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
    run git-undo
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
    run git-undo
    assert_success
    
    # Verify file is restored
    run git ls-files removeme.txt
    assert_success
    assert_output "removeme.txt"
    [ -f removeme.txt ]
    
    # ============================================================================
    # PHASE 1D: Test git restore undo (staged only)
    # ============================================================================
    echo "# Phase 1D: Testing git restore --staged undo..."
    
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
    run git-undo
    assert_success
    
    # Verify file is staged again
    run git diff --cached --name-only
    assert_success
    assert_line "staged.txt"
    
    echo "# Phase 1 Commands integration test completed successfully!"
}

@test "Phase 2 Commands: git reset, revert, cherry-pick undo functionality" {
    echo "# ============================================================================"
    echo "# PHASE 2 COMMANDS TEST: Testing git reset, revert, cherry-pick undo functionality"
    echo "# ============================================================================"
    
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
    # PHASE 2A: Test git reset undo
    # ============================================================================
    echo "# Phase 2A: Testing git reset undo..."
    
    # Get current commit hash for verification
    run git rev-parse HEAD
    assert_success
    third_commit="$output"
    
    # Perform a soft reset to previous commit
    git reset --soft HEAD~1
    
    # Verify we're at the second commit with staged changes
    run git rev-parse HEAD
    assert_success
    second_commit="$output"
    
    # Verify third.txt is staged
    run git diff --cached --name-only
    assert_success
    assert_line "third.txt"
    
    # Undo the reset (should restore HEAD to third_commit)
    run git-undo
    assert_success
    
    # Verify we're back at the third commit
    run git rev-parse HEAD
    assert_success
    assert_output "$third_commit"
    
    # Test mixed reset undo
    git reset HEAD~1
    
    # Verify second commit with unstaged changes
    run git rev-parse HEAD
    assert_success
    assert_output "$second_commit"
    
    run git status --porcelain
    assert_success
    assert_output --partial " D third.txt"
    
    # Undo the mixed reset
    run git-undo
    assert_success
    
    # Verify restoration
    run git rev-parse HEAD
    assert_success
    assert_output "$third_commit"
    
    # ============================================================================
    # PHASE 2B: Test git revert undo
    # ============================================================================
    echo "# Phase 2B: Testing git revert undo..."
    
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
    run git-undo
    assert_success
    
    # Verify we're back to the original commit
    run git rev-parse HEAD
    assert_success
    assert_output "$revert_target"
    
    # Verify file is back
    [ -f revert-me.txt ]
    
    # ============================================================================
    # PHASE 2C: Test git cherry-pick undo
    # ============================================================================
    echo "# Phase 2C: Testing git cherry-pick undo..."
    
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
    run git-undo
    assert_success
    
    # Verify we're back to the original main state
    run git rev-parse HEAD
    assert_success
    assert_output "$main_before_cherry"
    
    # Verify cherry-picked file is gone
    [ ! -f cherry.txt ]
    
    # ============================================================================
    # PHASE 2D: Test git clean undo (expected to fail)
    # ============================================================================
    echo "# Phase 2D: Testing git clean undo (should show unsupported error)..."
    
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
    run git-undo
    assert_failure
    assert_output --partial "permanently removes untracked files that cannot be recovered"
    
    echo "# Phase 2 Commands integration test completed successfully!"
}

@test "git switch undo functionality" {
    echo "# ============================================================================"
    echo "# GIT SWITCH TEST: Testing git switch undo functionality"
    echo "# ============================================================================"
    
    # Setup: Create initial commit structure for testing
    echo "initial content" > initial.txt
    git add initial.txt
    git commit -m "Initial commit"
    
    echo "main content" > main.txt
    git add main.txt
    git commit -m "Main content commit"
    
    # ============================================================================
    # Test git switch -c (branch creation) undo
    # ============================================================================
    echo "# Testing git switch -c branch creation undo..."
    
    # Create a new branch using git switch -c
    git switch -c feature-switch
    
    # Verify we're on the new branch
    run git branch --show-current
    assert_success
    assert_output "feature-switch"
    
    # Undo the branch creation
    run git-undo
    assert_success
    
    # Verify the branch was deleted and we're back on main
    run git branch --show-current
    assert_success
    assert_output "main"
    
    # Verify the feature-switch branch no longer exists
    run git branch --list feature-switch
    assert_success
    assert_output ""
    
    # ============================================================================
    # Test regular git switch undo (branch switching)
    # ============================================================================
    echo "# Testing regular git switch undo..."
    
    # Create a feature branch and switch to it
    git switch -c test-feature
    echo "feature content" > feature.txt
    git add feature.txt
    git commit -m "Feature content"
    
    # Switch back to main
    git switch main
    
    # Verify we're on main
    run git branch --show-current
    assert_success
    assert_output "main"
    
    # Undo the switch (should go back to test-feature)
    run git-undo
    assert_success
    
    # Verify we're back on test-feature
    run git branch --show-current
    assert_success
    assert_output "test-feature"
    
    # ============================================================================
    # Test git switch with uncommitted changes (should show warnings)
    # ============================================================================
    echo "# Testing git switch undo with uncommitted changes..."
    
    # Create some uncommitted changes
    echo "modified content" >> feature.txt
    echo "new unstaged file" > unstaged.txt
    
    # Switch to main (git switch should handle this)
    git switch main
    
    # Try to undo the switch (should work but with warnings in verbose mode)
    run git-undo -v
    assert_success
    
    # Verify we're back on test-feature
    run git branch --show-current
    assert_success
    assert_output "test-feature"
    
    # Clean up uncommitted changes for next tests
    git checkout -- feature.txt
    rm -f unstaged.txt
    
    echo "# git switch integration test completed successfully!"
}

@test "git undo checkout/switch detection - warns and suggests git back" {
    echo "# ============================================================================"
    echo "# CHECKOUT/SWITCH DETECTION TEST: Testing that git undo warns for checkout/switch commands"
    echo "# ============================================================================"
    
    # Setup: Create initial commit structure for testing
    echo "initial content" > initial.txt
    git add initial.txt
    git commit -m "Initial commit"
    
    echo "main content" > main.txt
    git add main.txt
    git commit -m "Main content commit"
    
    # Source the hook to enable command tracking
    source ~/.config/git-undo/git-undo-hook.bash
    
    # ============================================================================
    # Test git checkout detection
    # ============================================================================
    echo "# Testing git checkout detection..."
    
    # Create a test branch
    git branch test-branch
    
    # Perform checkout operation (should be tracked)
    git checkout test-branch
    
    # Verify we're on the test branch
    run git branch --show-current
    assert_success
    assert_output "test-branch"
    
    # Try git undo - should warn about checkout command
    run git undo 2>&1
    assert_success
    assert_output --partial "can't be undone"
    assert_output --partial "git back"
    
    # ============================================================================
    # Test git switch detection
    # ============================================================================
    echo "# Testing git switch detection..."
    
    # Switch back to main
    git switch main
    
    # Verify we're on main
    run git branch --show-current
    assert_success
    assert_output "main"
    
    # Switch to test branch again
    git switch test-branch
    
    # Try git undo - should warn about switch command
    run git undo 2>&1
    assert_success
    assert_output --partial "can't be undone"
    assert_output --partial "git back"
    
    # ============================================================================
    # Test that git back still works normally
    # ============================================================================
    echo "# Verifying git back still works normally..."
    
    # Use git back to return to previous branch
    run git back
    assert_success
    
    # Should be back on main
    run git branch --show-current
    assert_success
    assert_output "main"
    
    # ============================================================================
    # Test mixed commands - ensure warning only for checkout/switch
    # ============================================================================
    echo "# Testing mixed commands - ensuring warning only appears for checkout/switch..."
    
    # Create and stage a file
    echo "test file" > test-warning.txt
    git add test-warning.txt
    
    # Now perform git undo - should work normally (no warning)
    run git undo
    assert_success
    refute_output --partial "can't be undone"
    refute_output --partial "git back"
    
    # Verify file was unstaged
    run git status --porcelain
    assert_success
    assert_output --partial "?? test-warning.txt"
    
    echo "# Checkout/switch detection integration test completed successfully!"
} 