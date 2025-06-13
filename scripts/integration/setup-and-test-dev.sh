#!/bin/bash
set -euo pipefail

echo "DEV MODE: Installing git-undo from current source..."
# Enable dev mode and install from local source
export GIT_UNDO_DEV_MODE=true
export GIT_UNDO_TEST_MODE=true  # Use test hooks for integration tests
cd /home/testuser/git-undo-source
chmod +x install.sh
./install.sh

echo "Installation completed, setting up PATH and sourcing shell configuration..."
# Ensure Go binary path is in PATH BEFORE sourcing .bashrc (needed for hooks)
GOPATH_BIN="$(go env GOPATH)/bin"
export PATH="$GOPATH_BIN:$PATH"
# shellcheck disable=SC1090
source ~/.bashrc
cd /home/testuser

echo "Running integration tests..."
bats integration-test.bats 