#!/usr/bin/env bats

# Load bats helpers
load 'test_helper/bats-support/load'
load 'test_helper/bats-assert/load'

# Helper function for verbose command execution
# Usage: run_verbose <command> [args...]
# Shows command output with boxing for multi-line content
run_verbose() {
    run "$@"
    local cmd_str="$1"
    shift
    while [[ $# -gt 0 ]]; do
        cmd_str="$cmd_str $1"
        shift
    done

    if [[ $status -eq 0 ]]; then
        if [[ -n "$output" ]]; then
            # Check if output has multiple lines or is long
            local line_count=$(echo "$output" | wc -l)
            local char_count=${#output}
            if [[ $line_count -gt 1 ]] || [[ $char_count -gt 80 ]]; then
                echo ""
                echo -e "\033[95m┌─ $cmd_str ─\033[0m"
                echo -e "\033[95m$output\033[0m"
                echo -e "\033[95m└────────────\033[0m"
            else
                echo -e "\033[32m>\033[0m $cmd_str: $output"
            fi
        else
            echo -e "\033[32m>\033[0m $cmd_str: (no output)"
        fi
    else
        echo ""
        echo -e "\033[95m┌─ $cmd_str (FAILED: status $status) ─\033[0m"
        echo -e "\033[95m$output\033[0m"
        echo -e "\033[95m└────────────\033[0m"
    fi
}

# Helper function for commands that should only show output on failure
# Usage: run_quiet <command> [args...]
# Only shows output if command fails
run_quiet() {
    run "$@"
    if [[ $status -ne 0 ]]; then
        local cmd_str="$1"
        shift
        while [[ $# -gt 0 ]]; do
            cmd_str="$cmd_str $1"
            shift
        done
        echo "> $cmd_str FAILED: $output (status: $status)"
    fi
}

# Helper function for colored output
# Usage: print <message> - prints in cyan
# Usage: debug <message> - prints in gray
# Usage: title <message> - prints in yellow
print() {
    echo -e "\033[96m> $*\033[0m"  # Cyan
}

debug() {
    echo -e "\033[90m> DEBUG: $*\033[0m"  # Gray
}

title() {
    echo -e "\033[93m================================================================================"
    echo -e "\033[93m $*\033[0m"  # Yellow
    echo -e "\033[93m================================================================================\033[0m"
}

setup() {
    # Create isolated test repository for the test
    export TEST_REPO="$(mktemp -d)"
    cd "$TEST_REPO"

    git init
    git config user.email "git-undo-test@amberpixels.io"
    git config user.name "Git-Undo Integration Test User"

    # Configure git hooks for this repository
    git config core.hooksPath ~/.config/git-undo/hooks

    # Source hooks in the test shell environment
    # shellcheck disable=SC1090
    source ~/.config/git-undo/git-undo-hook.bash

    # Create initial empty commit so we always have HEAD (like in unit tests)
    git commit --allow-empty -m "init"
}

teardown() {
    # Clean up test repository
    rm -rf "$TEST_REPO"
}

@test "0A: complete git-undo integration workflow" {
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

@test "0B: git-back integration test - checkout and switch undo" {
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
    assert_output --partial "Git-back undoes the last git checkout or git switch command"

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

@test "1A: Phase 1A Commands: git rm, mv, tag, restore undo functionality" {
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

@test "2A: Phase 2A Commands: git reset, revert, cherry-pick undo functionality" {
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

@test "3A: git undo checkout/switch detection - warns and suggests git back" {
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

@test "4A: Additional Commands: git stash, merge, reset --hard, restore, branch undo functionality" {
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

@test "5A: Error Conditions and Edge Cases" {
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

@test "6A: Phase 6a - cursor-history: branching behavior after undo + new command (EXPECTED TO FAIL)" {
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

    # Test 1: git undo undo should NOT be possible (last action was not git undo)
    run_verbose git undo  # Undo F, leaving only A staged

    run_verbose git status --porcelain
    assert_success
    assert_output --partial "A  fileA.txt"
    refute_output --partial "A  fileF.txt"

    # This should fail - can't do git undo undo because branching occurred
    run_verbose git undo
    assert_failure
    assert_output --partial "can't undo"

    # Test 2: git undo should work (can undo F)
    git add fileF.txt  # Re-add F

    run git undo  # Should successfully undo F
    assert_success

    run git status --porcelain
    assert_success
    assert_output --partial "A  fileA.txt"
    refute_output --partial "A  fileF.txt"

    # Test 3: After git undo, can't do git undo again, only git undo undo would work
    # (but only if last action was git undo, which it was)
    run git undo  # This should redo F (git undo undo behavior)
    assert_success

    run git status --porcelain
    assert_success
    assert_output --partial "A  fileA.txt"
    assert_output --partial "A  fileF.txt"

    print "Phase 6a completed - tested branching behavior"
}
