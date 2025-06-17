#!/bin/bash
set -euo pipefail

echo "Downloading and running install.sh like a real user..."

export GIT_UNDO_TEST_MODE=true  # Use test hooks for integration tests
# Download and run install.sh exactly like real users do
curl -fsSL https://raw.githubusercontent.com/amberpixels/git-undo/main/install.sh | bash

echo "Installation completed, setting up PATH and sourcing shell configuration..."
# Ensure Go binary path is in PATH BEFORE sourcing .bashrc (needed for hooks)
GOPATH_BIN="$(go env GOPATH)/bin"
export PATH="$GOPATH_BIN:$PATH"
# shellcheck disable=SC1090
source ~/.bashrc

# Note: Git-undo hooks will be sourced within each BATS test's setup() function

echo "Running integration tests..."
bats integration-test.bats # --filter "3A:" # <- uncomment to run a specific test