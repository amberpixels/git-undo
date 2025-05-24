#!/usr/bin/env bash
set -e

source "$(dirname "$0")/common.sh"

install_shell_hook() {
    local shell_type="$1"
    local is_noop=true

    # Create config directory with proper permissions
    if [ ! -d "$CFG_DIR" ]; then
        mkdir -p "$CFG_DIR" 2>/dev/null || return 1
        chmod 755 "$CFG_DIR" 2>/dev/null || return 1
        is_noop=false
    fi

    case "$shell_type" in
        "zsh")
            local hook_file="git-undo-hook.zsh"
            local rc_file="$HOME/.zshrc"
            local source_line="source ~/.config/git-undo/$hook_file"

            # Copy the hook file and set permissions
            if [ ! -f "$ZSH_HOOK" ]; then
                cp "scripts/$hook_file" "$ZSH_HOOK" 2>/dev/null || return 1
                chmod 644 "$ZSH_HOOK" 2>/dev/null || return 1
                is_noop=false
            fi

            # Add source line to .zshrc if not already present
            if ! grep -qxF "$source_line" "$rc_file" 2>/dev/null; then
                echo "$source_line" >> "$rc_file" 2>/dev/null || return 1
                is_noop=false
            fi
            ;;

        "bash")
            local hook_file="git-undo-hook.bash"
            local source_line="source ~/.config/git-undo/$hook_file"

            # Copy the hook file and set permissions
            if [ ! -f "$BASH_HOOK" ]; then
                cp "scripts/$hook_file" "$BASH_HOOK" 2>/dev/null || return 1
                chmod 644 "$BASH_HOOK" 2>/dev/null || return 1
                is_noop=false
            fi

            # Determine which bash config file to use
            local rc_file
            if [[ "$OSTYPE" == "darwin"* ]]; then
                # macOS uses .bash_profile for login shells (default in Terminal.app)
                rc_file="$HOME/.bash_profile"
            else
                # Linux typically uses .bashrc for interactive shells
                rc_file="$HOME/.bashrc"
            fi

            # Add source line to the appropriate file if not already present
            if ! grep -qxF "$source_line" "$rc_file" 2>/dev/null; then
                echo "$source_line" >> "$rc_file" 2>/dev/null || return 1
                is_noop=false
            fi
            ;;

        *)
            return 1
            ;;
    esac

    # Return 2 if no changes were made (already installed)
    if $is_noop; then
        return 2
    fi
    return 0
}

main() {
    log "Starting installation..."

    # 1) Install the binary
    echo -en "${GRAY}git-undo:${RESET} 1. Installing Go binary..."

    # Check if we're in dev mode with local source available
    if [[ "${GIT_UNDO_DEV_MODE:-}" == "true" && -d "./cmd/git-undo" && -f "./Makefile" ]]; then
        echo -e " ${YELLOW}(dev mode)${RESET}"
        log "Building from local source using Makefile..."
        
        # Use Makefile's binary-install target which has proper version logic
        if make binary-install; then
            # Get the version that was just installed
            INSTALLED_VERSION=$(git-undo --version 2>/dev/null | grep -o 'git-undo.*' || echo "unknown")
            echo -e "${GRAY}git-undo:${RESET} Binary installed with version: ${BLUE}$INSTALLED_VERSION${RESET}"
        else
            echo -e "${GRAY}git-undo:${RESET} ${RED}Failed to build from source using Makefile${RESET}"
            exit 1
        fi
    else
        # Normal user installation from GitHub
        if go install -ldflags "-X main.version=$(get_latest_version)" "github.com/$REPO_OWNER/$REPO_NAME/cmd/git-undo@latest" 2>/dev/null; then
            echo -e " ${GREEN}OK${RESET}"
        else
            echo -e " ${RED}FAILED${RESET}"
            exit 1
        fi
    fi

    # 2) Shell integration
    local current_shell
    current_shell=$(detect_shell)
    echo -en "${GRAY}git-undo:${RESET} 2. Shell integration (${BLUE}$current_shell${RESET})..."
    
    # Temporarily disable set -e to capture non-zero exit codes
    set +e
    local hook_output
    hook_output=$(install_shell_hook "$current_shell" 2>&1)
    local hook_status=$?
    set -e
    
    case $hook_status in
        0)
            echo -e " ${GREEN}OK${RESET}"
            ;;
        2)
            echo -e " ${YELLOW}SKIP${RESET} (already configured)"
            ;;
        *)
            echo -e " ${RED}FAILED${RESET}"
            log "You can manually source the appropriate hook file from ${YELLOW}$CFG_DIR${RESET}"
            exit 1
            ;;
    esac

    # 3) Final message
    log "${GREEN}Installation completed successfully!${RESET}"
    echo -e ""
    echo -e "Please restart your shell or run '${YELLOW}source ~/.${current_shell}rc${RESET}' to activate ${BLUE}git-undo${RESET}"
}

main "$@"
