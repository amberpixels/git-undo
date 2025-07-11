#!/usr/bin/env bash

# Load bats assertion libraries
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

    # shellcheck disable=SC2154
    if [[ $status -eq 0 ]]; then
        if [[ -n "$output" ]]; then
            # Check if output has multiple lines or is long
            local line_count
            line_count=$(echo "$output" | wc -l)
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

# Common setup function for all tests
setup_git_undo_test() {
    # Create isolated test repository for the test
    TEST_REPO="$(mktemp -d)"
    export TEST_REPO
    cd "$TEST_REPO" || exit

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

# Common teardown function for all tests
teardown_git_undo_test() {
    # Clean up test repository
    rm -rf "$TEST_REPO"
}
