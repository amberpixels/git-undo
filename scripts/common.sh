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
VERSION_FILE="$CFG_DIR/version"

REPO_OWNER="amberpixels"
REPO_NAME="git-undo"
GITHUB_API_URL="https://api.github.com/repos/$REPO_OWNER/$REPO_NAME"
INSTALL_URL="https://raw.githubusercontent.com/$REPO_OWNER/$REPO_NAME/main/install.sh"

# ── shell detection ──────────────────────────────────────────────────────────
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

get_current_version() {
    if [[ -f "$VERSION_FILE" ]]; then
        cat "$VERSION_FILE"
    else
        echo "unknown"
    fi
}

extract_tag_version() {
    local version="$1"
    # Extract just the tag part from git describe output (e.g., v0.0.1-10-g4dd7da9 -> v0.0.1)
    echo "$version" | sed 's/-[0-9]*-g[0-9a-f]*$//'
}

get_current_tag_version() {
    # First try to get version from git if we're in a git repo
    if [[ -d ".git" ]]; then
        local git_version
        git_version=$(git describe --tags --exact-match 2>/dev/null || git describe --tags 2>/dev/null || echo "")
        if [[ -n "$git_version" ]]; then
            extract_tag_version "$git_version"
            return
        fi
    fi
    
    # Fallback to stored version file
    local version
    version=$(get_current_version)
    if [[ "$version" == "unknown" ]]; then
        echo "unknown"
    else
        extract_tag_version "$version"
    fi
}

set_current_version() {
    local version="$1"
    echo "$version" > "$VERSION_FILE"
}

get_latest_version() {
    local latest_release
    if command -v curl >/dev/null 2>&1; then
        latest_release=$(curl -s "$GITHUB_API_URL/releases/latest" | grep '"tag_name":' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        latest_release=$(wget -qO- "$GITHUB_API_URL/releases/latest" | grep '"tag_name":' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/')
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
    
    # Convert versions to comparable format (e.g., 1.2.3 -> 001002003)
    local v1=$(echo "$version1" | awk -F. '{ printf("%03d%03d%03d\n", $1, $2, $3); }')
    local v2=$(echo "$version2" | awk -F. '{ printf("%03d%03d%03d\n", $1, $2, $3); }')
    
    if [[ "$v1" < "$v2" ]]; then
        echo "older"
    elif [[ "$v1" > "$v2" ]]; then
        echo "newer"
    else
        echo "same"
    fi
} 