#!/usr/bin/env bash

# Test script to demonstrate duplicate prevention between shell and git hooks

set -e

echo "=== Testing duplicate prevention mechanism ==="
echo

# Initialize a test repo if not already
if [[ ! -d .git ]]; then
    echo "Initializing test git repo..."
    git init
    git config user.name "Test User" 
    git config user.email "test@example.com"
fi

# Clear any existing log
rm -f .git/git-undo/commands .git/git-undo/.shell-hook-*

echo "1. Testing shell hook logging (simulating shell hook call):"
echo "   GIT_UNDO_INTERNAL_HOOK=1 ./git-undo --hook='git commit -m \"test commit\"'"
GIT_UNDO_INTERNAL_HOOK=1 ./git-undo --hook='git commit -m "test commit"'
echo "   Shell hook logged the command"
echo

echo "2. Testing git hook logging immediately after (simulating git hook call):"
echo "   GIT_UNDO_GIT_HOOK_MARKER=1 GIT_HOOK_NAME=post-commit GIT_UNDO_INTERNAL_HOOK=1 ./git-undo --hook='git commit -m \"test commit\"'"
GIT_UNDO_GIT_HOOK_MARKER=1 GIT_HOOK_NAME=post-commit GIT_UNDO_INTERNAL_HOOK=1 ./git-undo --hook='git commit -m "test commit"'
echo "   Git hook should have detected the duplicate and skipped logging"
echo

echo "3. Current log contents:"
if [[ -f .git/git-undo/commands ]]; then
    cat .git/git-undo/commands
    echo
    echo "Lines in log: $(wc -l < .git/git-undo/commands)"
else
    echo "   No log file found"
fi
echo

echo "4. Testing git hook logging alone (after cleanup):"
# Clean up shell hook markers
rm -f .git/git-undo/.shell-hook-*
sleep 1
echo "   GIT_UNDO_GIT_HOOK_MARKER=1 GIT_HOOK_NAME=post-commit GIT_UNDO_INTERNAL_HOOK=1 ./git-undo --hook='git commit -m \"another commit\"'"
GIT_UNDO_GIT_HOOK_MARKER=1 GIT_HOOK_NAME=post-commit GIT_UNDO_INTERNAL_HOOK=1 ./git-undo --hook='git commit -m "another commit"'
echo "   Git hook should have logged since no shell hook marker exists"
echo

echo "5. Final log contents:"
if [[ -f .git/git-undo/commands ]]; then
    cat .git/git-undo/commands
    echo
    echo "Lines in log: $(wc -l < .git/git-undo/commands)"
else
    echo "   No log file found"
fi

echo
echo "=== Test complete ==="
echo "Expected result: 2 log entries (one from shell hook, one from git hook when shell hook wasn't active)" 