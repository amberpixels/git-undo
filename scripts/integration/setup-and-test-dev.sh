#!/bin/bash
set -euo pipefail

echo "DEV MODE: Installing git-undo from current source..."
# Enable dev mode and install from local source
export GIT_UNDO_DEV_MODE=true
cd /home/testuser/git-undo-source
chmod +x install.sh
./install.sh

echo "Installation completed, setting up PATH and sourcing shell configuration..."
# Ensure Go binary path is in PATH BEFORE sourcing .bashrc (needed for hooks)
export PATH="$(go env GOPATH)/bin:$PATH"
source ~/.bashrc
cd /home/testuser

echo "Running integration tests..."
bats integration-test.bats 