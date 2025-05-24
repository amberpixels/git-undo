#!/bin/bash
set -euo pipefail

echo "Downloading and running install.sh like a real user..."

export GIT_UNDO_TEST_MODE=true  # Use test hooks for integration tests
# Download and run install.sh exactly like real users do
curl -fsSL https://raw.githubusercontent.com/amberpixels/git-undo/main/install.sh | bash

echo "Installation completed, setting up PATH and sourcing shell configuration..."
# Ensure Go binary path is in PATH BEFORE sourcing .bashrc (needed for hooks)
export PATH="$(go env GOPATH)/bin:$PATH"
source ~/.bashrc

echo "Running integration tests..."
bats integration-test.bats 