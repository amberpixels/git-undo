#!/usr/bin/env bash

# Source shared colors and basic logging
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/colors.sh"

# Git-undo specific configuration
BIN_NAME="git-undo"
BIN_DIR=$(go env GOBIN 2>/dev/null || true)
[[ -z "$BIN_DIR" ]] && BIN_DIR="$(go env GOPATH)/bin"
BIN_PATH="$BIN_DIR/$BIN_NAME"

CFG_DIR="$HOME/.config/git-undo"
ZSH_HOOK="$CFG_DIR/git-undo-hook.zsh"
BASH_HOOK="$CFG_DIR/git-undo-hook.bash"
VERSION_FILE="$CFG_DIR/version"

REPO_OWNER="amberpixels"
REPO_NAME="git-undo"
GITHUB_API_URL="https://api.github.com/repos/$REPO_OWNER/$REPO_NAME"
INSTALL_URL="https://raw.githubusercontent.com/$REPO_OWNER/$REPO_NAME/main/install.sh"

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

get_latest_version() {
    local latest_release
    if command -v curl >/dev/null 2>&1; then
        latest_release=$(curl -s "$GITHUB_API_URL/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        latest_release=$(wget -qO- "$GITHUB_API_URL/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        echo "error: curl or wget required for version check" >&2
        return 1
    fi
    
    if [[ -z "$latest_release" || "$latest_release" == "null" ]]; then
        echo "error: failed to fetch latest version" >&2
        return 1
    fi
    
    echo "$latest_release"
}

version_compare() {
    local version1="$1"
    local version2="$2"
    
    # Remove 'v' prefix if present
    version1=${version1#v}
    version2=${version2#v}
    
    # Extract base version (everything before the first dash)
    local base1=$(echo "$version1" | cut -d'-' -f1)
    local base2=$(echo "$version2" | cut -d'-' -f1)
    
    # Convert base versions to comparable format (e.g., 1.2.3 -> 001002003)
    local v1=$(echo "$base1" | awk -F. '{ printf("%03d%03d%03d\n", $1, $2, $3); }')
    local v2=$(echo "$base2" | awk -F. '{ printf("%03d%03d%03d\n", $1, $2, $3); }')
    
    # Compare base versions first
    if [[ "$v1" < "$v2" ]]; then
        echo "older"
    elif [[ "$v1" > "$v2" ]]; then
        echo "newer"
    else
        # Base versions are the same, check for development version indicators
        # If one has additional info (date, commit, branch) and the other doesn't, 
        # the one with additional info is newer
        if [[ "$version1" == "$base1" && "$version2" != "$base2" ]]; then
            # version1 is base tag, version2 is development version
            echo "older"
        elif [[ "$version1" != "$base1" && "$version2" == "$base2" ]]; then
            # version1 is development version, version2 is base tag
            echo "newer"
        else
            # Both are either base tags or both are development versions
            echo "same"
        fi
    fi
} 