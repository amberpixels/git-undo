#!/usr/bin/env bats

load 'test_helpers'

setup() {
    setup_git_undo_test
}

teardown() {
    teardown_git_undo_test
}

@test "0__A: complete git-undo integration workflow" {
    # ============================================================================
    # PHASE 0A-1: Verify Installation
    # ============================================================================
    title "Phase 0A-1: Verifying git-undo installation..."

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
    title "HOOK DIAGNOSTICS: Checking hook installation..."

    # Check if hook files exist
    echo "> Checking if hook files exist in ~/.config/git-undo/..."
    run ls -la ~/.config/git-undo/
    assert_success
    echo "> Hook directory contents: ${output}"

    # Verify hook files are present
    assert [ -f ~/.config/git-undo/git-undo-hook.bash ]
    echo "> ✓ Hook file exists: ~/.config/git-undo/git-undo-hook.bash"

    # Verify that the test hook is actually installed (should contain git function)
    echo "> Checking if test hook is installed (contains git function)..."
    run grep -q "git()" ~/.config/git-undo/git-undo-hook.bash
    assert_success
    echo "> ✓ Test hook confirmed: contains git function"

    # Check if .bashrc has the source line
    echo "> Checking if .bashrc contains git-undo source line..."
    run grep -n git-undo ~/.bashrc
    assert_success
    echo "> .bashrc git-undo lines: ${output}"

    # Check current git command type (before sourcing hooks)
    echo "> Checking git command type before hook loading..."
    run type git
    echo "> Git type before: ${output}"

    # Check git command type (hooks are sourced in setup)
    echo "> Checking git command type after hook loading..."
    run type git
    echo "> Git type after: ${output}"

    # Test if git-undo function/alias is available
    echo "> Testing if git undo command is available..."
    run git undo --help
    if [[ $status -eq 0 ]]; then
        echo "> ✓ git undo command responds"
    else
        echo "> ✗ git undo command failed with status: $status"
        echo "> Output: ${output}"
    fi

    # ============================================================================
    # PHASE 0A-2: Basic git add and undo workflow
    # ============================================================================
    title "Phase 0A-2: Testing basic git add and undo..."

    # Create test files
    echo "content of file1" > file1.txt
    echo "content of file2" > file2.txt
    echo "content of file3" > file3.txt

    # Verify files are untracked initially
    run_verbose git status --porcelain
    assert_success
    assert_output --partial "?? file1.txt"
    assert_output --partial "?? file2.txt"
    assert_output --partial "?? file3.txt"

    # Add first file
    git add file1.txt
    run_verbose git status --porcelain
    assert_success
    assert_output --partial "A  file1.txt"
    assert_output --partial "?? file2.txt"
    assert_output --partial "?? file3.txt"

    # Add second file
    git add file2.txt
    run_verbose git status --porcelain
    assert_success
    assert_output --partial "A  file1.txt"
    assert_output --partial "A  file2.txt"
    assert_output --partial "?? file3.txt"

    # First undo - should unstage file2.txt
    debug "Checking git-undo log before first undo..."
    run_verbose git undo --log
    assert_success
    refute_output ""  # Log should not be empty if hooks are tracking

    run git undo
    assert_success

    run_verbose git status --porcelain
    assert_success
    assert_output --partial "A  file1.txt"
    assert_output --partial "?? file2.txt"
    assert_output --partial "?? file3.txt"

    # Second undo - should unstage file1.txt
    debug "Checking git-undo log before second undo..."
    run_verbose git undo --log
    assert_success
    refute_output ""  # Log should not be empty if hooks are tracking

    run git undo
    assert_success

    run_verbose git status --porcelain
    assert_success
    assert_output --partial "?? file1.txt"
    assert_output --partial "?? file2.txt"
    assert_output --partial "?? file3.txt"
    refute_output --partial "A  file1.txt"
    refute_output --partial "A  file2.txt"

    # ============================================================================
    # PHASE 0A-3: Commit and undo workflow
    # ============================================================================
    title "Phase 0A-3: Testing commit and undo..."

    # Stage and commit first file
    git add file1.txt
    git commit -m "Add file1.txt"

    # Verify clean working directory (except for untracked files from previous phase)
    run_verbose git status --porcelain
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
    run_verbose git status --porcelain
    assert_success
    assert_output --partial "?? file3.txt"
    refute_output --partial "file1.txt"  # file1.txt should be committed
    refute_output --partial "file2.txt"  # file2.txt should be committed

    # First commit undo - should undo last commit, leaving file2 staged
    debug "Checking git-undo log before commit undo..."
    run_verbose git undo --log
    assert_success
    refute_output ""  # Log should not be empty if hooks are tracking

    run git undo
    assert_success

    run_verbose git status --porcelain
    assert_success
    assert_output --partial "A  file2.txt"

    # Verify files still exist in working directory
    assert [ -f "file1.txt" ]
    assert [ -f "file2.txt" ]

    # Second undo - should unstage file2.txt
    debug "Checking git-undo log before second commit undo..."
    run_verbose git undo --log
    assert_success
    refute_output ""  # Log should not be empty if hooks are tracking

    run git undo
    assert_success

    run_verbose git status --porcelain
    assert_success
    assert_output --partial "?? file2.txt"
    assert_output --partial "?? file3.txt"
    refute_output --partial "A  file2.txt"

    # ============================================================================
    # PHASE 0A-4: Complex sequential workflow
    # ============================================================================
    title "Phase 0A-4: Testing complex sequential operations..."

    # Commit file3
    git add file3.txt
    git commit -m "Add file3.txt"

    # Modify file1 and stage the modification
    echo "modified content" >> file1.txt
    git add file1.txt

    # Verify modified file1 is staged
    run_verbose git status --porcelain
    assert_success
    assert_output --partial "M  file1.txt"

    # Create and stage a new file
    echo "content of file4" > file4.txt
    git add file4.txt

    # Verify both staged
    run_verbose git status --porcelain
    assert_success
    assert_output --partial "M  file1.txt"
    assert_output --partial "A  file4.txt"

    # Undo staging of file4
    debug "Checking git-undo log before file4 undo..."
    run_verbose git undo --log
    assert_success
    refute_output ""  # Log should not be empty if hooks are tracking

    run git undo
    assert_success

    run_verbose git status --porcelain
    assert_success
    assert_output --partial "M  file1.txt"  # file1 still staged
    assert_output --partial "?? file4.txt"  # file4 unstaged
    refute_output --partial "A  file4.txt"

    # Undo staging of modified file1
    debug "Checking git-undo log before modified file1 undo..."
    run_verbose git undo --log
    assert_success
    refute_output ""  # Log should not be empty if hooks are tracking

    run git undo
    assert_success

    run_verbose git status --porcelain
    assert_success
    assert_output --partial " M file1.txt"  # Modified but unstaged
    assert_output --partial "?? file4.txt"
    refute_output --partial "M  file1.txt"  # Should not be staged anymore

    # Undo commit of file3
    run git undo
    assert_success

    run_verbose git status --porcelain
    assert_success
    assert_output --partial "A  file3.txt"  # file3 back to staged
    assert_output --partial " M file1.txt"  # file1 still modified
    assert_output --partial "?? file4.txt"

    # ============================================================================
    # PHASE 0A-5: Verification of final state
    # ============================================================================
    title "Phase 0A-5: Final state verification..."

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

    print "Integration test completed successfully!"
}

@test "0__B: git-back integration test - checkout and switch undo" {
    # ============================================================================
    # PHASE 0B-1: Verify git-back Installation
    # ============================================================================
    title "Phase 0B-1: Verifying git-back installation..."

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
    assert_output --partial "Navigate back through git checkout/switch operations"

    # ============================================================================
    # PHASE 0B-2: Basic branch switching workflow
    # ============================================================================
    title "Phase 0B-2: Testing basic branch switching..."

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
    # PHASE 0B-3: Test git-back with checkout commands
    # ============================================================================
    title "Phase 0B-3: Testing git-back with checkout..."

    # Switch to feature branch (this should be tracked)
    git checkout feature-branch

    # Verify we're on feature-branch
    run git branch --show-current
    assert_success
    assert_output "feature-branch"

    # Use git-back to go back to previous branch (should be another-branch)
    run_verbose git back
    assert_success

    # Verify we're back on another-branch
    run git branch --show-current
    assert_success
    assert_output "another-branch"

    # ============================================================================
    # PHASE 0B-4: Test multiple branch switches
    # ============================================================================
    title "Phase 0B-4: Testing multiple branch switches..."

    # Switch to main branch
    git checkout main

    # Verify we're on main
    run git branch --show-current
    assert_success
    assert_output "main"

    # Use git-back to go back to previous branch (should be another-branch)
    run_verbose git back
    assert_success

    # Verify we're back on another-branch
    run git branch --show-current
    assert_success
    assert_output "another-branch"

    # Switch to feature-branch again
    git checkout feature-branch

    debug "Checking git-undo log before modified file1 undo..."
    run_verbose git undo --log
    assert_success
    refute_output ""  # Log should not be empty if hooks are tracking

    # Use git-back to go back to another-branch
    run_verbose git back
    assert_success

    # Verify we're on another-branch
    run git branch --show-current
    assert_success
    assert_output "another-branch"

    # ============================================================================
    # PHASE 0B-5: Test git-back with uncommitted changes (should show warnings)
    # ============================================================================
    title "Phase 0B-5: Testing git-back with uncommitted changes..."

    # Make some uncommitted changes
    echo "modified content" >> another.txt
    echo "new file content" > unstaged.txt

    # Stage one file
    git add unstaged.txt

    # Now try git-back in verbose mode to see warnings
    run_verbose git undo --log

    # Now try git-back in verbose mode to see warnings
    run_verbose git back -v
    # Note: This might fail due to conflicts, but we want to verify warnings are shown
    # The important thing is that warnings are displayed to the user
    
    # Verify that the failure message is shown
    assert_output --partial "failed to execute"

    # For testing purposes, let's stash the changes and try again
    git stash

    # Now git-back should work
    run_verbose git back
    assert_success

    # Verify we're back on feature-branch
    run git branch --show-current
    assert_success
    assert_output "feature-branch"

    print "git-back integration test completed successfully!"
}
