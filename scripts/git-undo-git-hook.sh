#!/usr/bin/env bash

# This script is called by git hooks to notify git-undo of git operations.
set -euo pipefail

hook_name=$(basename "$0")   # post-commit / post-merge
cmd=""

# Check if git-undo is available
# silently exit if not available
if ! command -v git-undo >/dev/null 2>&1; then
    exit 0
fi

case "$hook_name" in
    post-commit)
        # Reconstruct something close to what the user typed.
        # We can’t know flags, so keep it minimal.
        # Escape quotes in commit message for safe shell execution

        commit_msg=$(git log -1 --pretty=format:'%s' 2>/dev/null || echo "commit")
        commit_msg=$(printf '%s\n' "$commit_msg" | sed 's/"/\\"/g')
        cmd="git commit -m \"$commit_msg\""
        ;;
    post-merge)
        # $1 contains the squash-merge flag (1 for squash, 0 for regular merge)
        if [[ "${1:-0}" == "1" ]]; then
            cmd="git merge --squash"
        else
            # Try to determine if it was a fast-forward merge
            # This is approximate since we can't know the exact command used
            if git log -1 --pretty=format:'%P' | grep -q ' '; then
                # Merge commit (has multiple parents)
                cmd="git merge --no-ff"
            else
                # Fast-forward merge (single parent)
                cmd="git merge --ff"
            fi
        fi
        ;;
    *)
        exit 0        # unknown hook → ignore
        ;;
esac

# Set markers to help git-undo distinguish git hooks from shell hooks
export GIT_UNDO_GIT_HOOK_MARKER=1
export GIT_HOOK_NAME="$hook_name"

# Re-use your existing internal flag so the Go binary accepts the call.
GIT_UNDO_INTERNAL_HOOK=1 exec git-undo --hook="$cmd" 2>/dev/null || true
