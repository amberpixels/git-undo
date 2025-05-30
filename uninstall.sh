#!/usr/bin/env bash
# This file is auto-generated by scripts/build.sh
# DO NOT EDIT - modify scripts/*.src.sh instead and run 'make buildscripts'
set -euo pipefail

# ── Inlined content from common.sh ──────────────────────────────────────────

# Color definitions - shared across all scripts
GRAY='\033[90m'
GREEN='\033[32m'
YELLOW='\033[33m'
RED='\033[31m'
BLUE='\033[34m'
RESET='\033[0m'

# Alternative name for compatibility
NC="$RESET"  # No Color (used in some scripts)

# Basic logging functions
log() { 
    echo -e "${GRAY}git-undo:${RESET} $1"
}

log_info() {
    echo -e "${BLUE}[INFO]${RESET} $*"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${RESET} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${RESET} $*"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${RESET} $*"
} 


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
GITHUB_REPO_URL="github.com/$REPO_OWNER/$REPO_NAME"
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
# ── End of inlined content ──────────────────────────────────────────────────

scrub_rc() {
    local rc="$1"
    [[ -e "$rc" ]] || return 1
    local real_rc="$rc"
    [[ -L "$rc" ]] && real_rc="$(readlink -f "$rc")"

    [[ -f "$real_rc" ]] || return 1

    # Check if hook line exists before attempting to remove it
    if ! grep -q "source .*git-undo-hook" "$real_rc" 2>/dev/null; then
        return 1  # No hook line found, nothing to do
    fi

    # Create backup only if we're going to modify the file
    cp "$real_rc" "${real_rc}.bak.$(date +%s)"

    # cross-platform sed in-place
    if sed --version &>/dev/null; then                    # GNU
        sed -i "/source .*git-undo-hook/d" "$real_rc"
    else                                                  # BSD / macOS
        sed -i '' "/source .*git-undo-hook/d" "$real_rc"
    fi
    return 0  # Successfully cleaned
}

main() {
    log "Starting uninstallation..."

    # 1) Remove binary
    echo -en "${GRAY}git-undo:${RESET} 1. Removing binary..."
    if [[ -f "$BIN_PATH" ]]; then
        rm -f "$BIN_PATH"
        echo -e " ${GREEN}OK${RESET}"
    else
        echo -e " ${YELLOW}SKIP${RESET} (not found)"
    fi

    # 2) Clean shell configuration files
    echo -en "${GRAY}git-undo:${RESET} 2. Cleaning shell configurations..."
    local cleaned_files=0
    
    # Check each rc file and count successful cleanings
    scrub_rc "$HOME/.zshrc" && ((cleaned_files++)) || true
    scrub_rc "$HOME/.bashrc" && ((cleaned_files++)) || true
    scrub_rc "$HOME/.bash_profile" && ((cleaned_files++)) || true
    
    if [ $cleaned_files -gt 0 ]; then
        echo -e " ${GREEN}OK${RESET} ($cleaned_files files)"
    else
        echo -e " ${YELLOW}SKIP${RESET} (no hook lines found)"
    fi

    # 3) Remove config directory
    echo -en "${GRAY}git-undo:${RESET} 3. Removing config directory..."
    if [[ -d "$CFG_DIR" ]]; then
        rm -rf "$CFG_DIR"
        echo -e " ${GREEN}OK${RESET}"
    else
        echo -e " ${YELLOW}SKIP${RESET} (not found)"
    fi

    # 4) Final message
    log "${GREEN}Uninstallation completed successfully!${RESET}"
}

main "$@"
