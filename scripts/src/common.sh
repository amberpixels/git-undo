#!/usr/bin/env bash

# _get_script_dir determines the directory of this file (common.sh) in a POSIX-portable way
#
# Works when:
#   • the script is executed directly (`bash install.sh`)
#   • the script is sourced from another Bash script
#   • the outer shell is zsh (she-bang ensures Bash runs underneath)
_get_script_dir() {
    # $1 = one element of ${BASH_SOURCE[@]} or the zsh %x expansion
    local src="$1"
    while [ -h "$src" ]; do # Resolve symlinks if any
        local dir
        dir="$(cd -P -- "$(dirname -- "$src")" && pwd)"
        src="$(readlink "$src")"
        [[ $src != /* ]] && src="$dir/$src"
    done

    # physical directory of the file itself
    local dir
    dir=$(cd -P -- "$(dirname -- "$src")" && pwd)
    # Since common.sh is now in scripts/src/, we need to go up one level to get scripts/
    # Remove /src suffix if present, otherwise assume we're already in scripts/
    if [[ "$dir" == */src ]]; then
        printf '%s' "${dir%/src}"
    else
        printf '%s' "$dir"
    fi
}

if [[ -n "${BASH_SOURCE[0]:-}" ]]; then               # Bash (she-bang path)
    SCRIPT_DIR="$(_get_script_dir "${BASH_SOURCE[0]}")"
else                                                  # POSIX sh execution
    SCRIPT_DIR="$(_get_script_dir "$0")"
fi
unset -f _get_script_dir

echo "SCRIPT DIR $SCRIPT_DIR"
# Coloring helpers
# shellcheck disable=SC1091
source "$SCRIPT_DIR/src/colors.sh"

# Git-undo specific configuration
UNDO_BIN_NAME="git-undo"
BACK_BIN_NAME="git-back"
BIN_DIR=$(go env GOBIN 2>/dev/null || true)
[[ -z "$BIN_DIR" ]] && BIN_DIR="$(go env GOPATH)/bin"
export UNDO_BIN_PATH="$BIN_DIR/$UNDO_BIN_NAME"
export BACK_BIN_PATH="$BIN_DIR/$BACK_BIN_NAME"

# Legacy variable for backward compatibility
export BIN_PATH="$UNDO_BIN_PATH"

CFG_DIR="$HOME/.config/git-undo"
export BASH_HOOK="$CFG_DIR/git-undo-hook.bash"
export ZSH_HOOK="$CFG_DIR/git-undo-hook.zsh"
GIT_HOOKS_DIR="$CFG_DIR/hooks"
DISPATCHER_FILE="$GIT_HOOKS_DIR/git-hooks.sh"
DISPATCHER_SRC="$SCRIPT_DIR/scripts/git-undo-git-hook.sh"

REPO_OWNER="amberpixels"
REPO_NAME="git-undo"
export GITHUB_REPO_URL="github.com/$REPO_OWNER/$REPO_NAME"
GITHUB_API_URL="https://api.github.com/repos/$REPO_OWNER/$REPO_NAME"
export INSTALL_URL="https://raw.githubusercontent.com/$REPO_OWNER/$REPO_NAME/main/install.sh"

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
    local base1
    local base2
    base1=$(echo "$version1" | cut -d'-' -f1)
    base2=$(echo "$version2" | cut -d'-' -f1)

    # Convert base versions to comparable format (e.g., 1.2.3 -> 001002003)
    local v1
    local v2
    v1=$(echo "$base1" | awk -F. '{ printf("%03d%03d%03d\n", $1, $2, $3); }')
    v2=$(echo "$base2" | awk -F. '{ printf("%03d%03d%03d\n", $1, $2, $3); }')

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

install_dispatcher_into() {
    local target="$1"

    # Validate target directory
    if [[ -z "$target" ]]; then
        log_error "Target directory not specified"
        return 1
    fi

    log "Installing git hooks into: $target"

    # Create target directory if it doesn't exist
    if ! mkdir -p "$target" 2>/dev/null; then
        log_error "Failed to create hooks directory: $target"
        return 1
    fi

    # 1) Install the dispatcher script only if target is our managed hooks directory
    if [[ "$target" == "$GIT_HOOKS_DIR" ]]; then
        log "Installing dispatcher script to: $DISPATCHER_FILE"

        # Debug: Check if source file exists
        if [[ ! -f "$DISPATCHER_SRC" ]]; then
            log_error "Source dispatcher script not found: $DISPATCHER_SRC"
            log_error "DISPATCHER_SRC variable: '$DISPATCHER_SRC'"
            log_error "Contents of script directory:"
            ls -la "$(dirname "$DISPATCHER_SRC")" 2>/dev/null || log_error "Cannot list script directory"
            return 1
        fi

        log "Source file exists: $DISPATCHER_SRC"
        log "Source file permissions: $(ls -l "$DISPATCHER_SRC")"

        # Ensure the hooks directory exists
        if ! mkdir -p "$GIT_HOOKS_DIR" 2>/dev/null; then
            log_error "Failed to create git hooks directory: $GIT_HOOKS_DIR"
            return 1
        fi

        log "Target directory exists: $GIT_HOOKS_DIR"
        log "Target directory permissions: $(ls -ld "$GIT_HOOKS_DIR")"

        # Try using cp instead of install command for better compatibility
        if cp "$DISPATCHER_SRC" "$DISPATCHER_FILE" 2>/dev/null; then
            chmod 755 "$DISPATCHER_FILE" 2>/dev/null || {
                log_error "Failed to set permissions on dispatcher script"
                return 1
            }
            log "Dispatcher copied and permissions set successfully"
        else
            log_error "Failed to copy dispatcher script from $DISPATCHER_SRC to $DISPATCHER_FILE"
            log_error "Let's try to understand why:"

            # More detailed debugging
            log_error "Source file readable? $(test -r "$DISPATCHER_SRC" && echo "YES" || echo "NO")"
            log_error "Target directory writable? $(test -w "$GIT_HOOKS_DIR" && echo "YES" || echo "NO")"
            log_error "Disk space available? $(df -h "$GIT_HOOKS_DIR" | tail -1)"

            return 1
        fi
    fi

    # 2) Wire up post-commit & post-merge hooks
    for hook in post-commit post-merge; do
        local hook_file="$target/$hook"
        log "Processing hook: $hook_file"

        if [[ -f "$hook_file" && ! -L "$hook_file" ]]; then
            # Existing regular file - append our hook call if not already present
            log "Found existing hook file, checking if git-undo is already integrated"

            if ! grep -q 'git-undo --hook' "$hook_file" 2>/dev/null; then
                log "Adding git-undo integration to existing hook"
                {
                    echo ""
                    echo "# git-undo integration"
                    echo "GIT_UNDO_INTERNAL_HOOK=1 git-undo --hook=\"$hook\""
                } >> "$hook_file"

                # Ensure it's executable
                chmod +x "$hook_file" 2>/dev/null || {
                    log_error "Failed to make hook executable: $hook_file"
                    return 1
                }
                log "Successfully integrated with existing $hook hook"
            else
                log "git-undo already integrated in $hook hook"
            fi

        elif [[ -L "$hook_file" ]]; then
            # It's a symlink - check if it points to our dispatcher
            local link_target
            link_target=$(readlink "$hook_file" 2>/dev/null || echo "")

            if [[ "$link_target" != "$DISPATCHER_FILE" ]]; then
                log_warning "Hook $hook_file is a symlink to $link_target, not our dispatcher"
                log "This hook may not work with git-undo"
            else
                log "Hook $hook_file already points to our dispatcher"
            fi

        else
            # No hook exists yet - create one
            log "Creating new $hook hook"

            # Try to create a symlink first (preferred method)
            if ln -sf "$DISPATCHER_FILE" "$hook_file" 2>/dev/null; then
                log "Created symlink: $hook_file -> $DISPATCHER_FILE"
            else
                # Fallback for filesystems that don't support symlinks
                log "Symlink failed, creating standalone hook script"
                cat > "$hook_file" << EOF
#!/usr/bin/env bash
# git-undo hook - auto-generated
set -e
GIT_UNDO_INTERNAL_HOOK=1 git-undo --hook="$hook"
EOF
                chmod +x "$hook_file" 2>/dev/null || {
                    log_error "Failed to make hook executable: $hook_file"
                    return 1
                }
                log "Created standalone hook: $hook_file"
            fi
        fi
    done

    log "Hook installation completed for: $target"
    return 0
}
