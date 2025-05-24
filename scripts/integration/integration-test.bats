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
    assert_output --partial "git-undo"
    
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