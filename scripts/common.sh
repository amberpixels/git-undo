#!/usr/bin/env bash

GRAY='\033[90m'; GREEN='\033[32m'; YELLOW='\033[33m'; RED='\033[31m'; BLUE='\033[34m'; RESET='\033[0m'
log() { echo -e "${GRAY}git-undo:${RESET} $1"; }

BIN_NAME="git-undo"
BIN_DIR=$(go env GOBIN 2>/dev/null || true)
[[ -z "$BIN_DIR" ]] && BIN_DIR="$(go env GOPATH)/bin"
BIN_PATH="$BIN_DIR/$BIN_NAME"

CFG_DIR="$HOME/.config/git-undo"
ZSH_HOOK="$CFG_DIR/git-undo-hook.zsh"
BASH_HOOK="$CFG_DIR/git-undo-hook.bash"

detect_shell() {
    # Method 1: Check $SHELL environment variable (most reliable for login shell)
    if [[ -n "$SHELL" ]]; then
        case "$SHELL" in
            *zsh*)
                echo "zsh"
                return
                ;;
            *bash*)
                echo "bash"
                return
                ;;
        esac
    fi

    # Method 2: Check shell-specific version variables
    if [[ -n "$ZSH_VERSION" ]]; then
        echo "zsh"
        return
    elif [[ -n "$BASH_VERSION" ]]; then
        echo "bash"
        return
    fi

    # If all methods fail
    echo "unknown"
} 