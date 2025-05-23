#!/usr/bin/env bash
set -e

GRAY='\033[90m'; GREEN='\033[32m'; YELLOW='\033[33m'; RED='\033[31m'; BLUE='\033[34m'; RESET='\033[0m'
log()  { echo -e "${GRAY}git-undo:${RESET} $1"; }

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

# Function to install shell hook
install_shell_hook() {
    local shell_type="$1"
    local config_dir="$HOME/.config/git-undo"
    local is_noop=true

    # Create config directory with proper permissions
    if [ ! -d "$config_dir" ]; then
        mkdir -p "$config_dir" 2>/dev/null || return 1
        chmod 755 "$config_dir" 2>/dev/null || return 1
        is_noop=false
    fi

    case "$shell_type" in
        "zsh")
            local hook_file="git-undo-hook.zsh"
            local rc_file="$HOME/.zshrc"
            local source_line="source ~/.config/git-undo/$hook_file"

            # Copy the hook file and set permissions
            if [ ! -f "$config_dir/$hook_file" ]; then
                cp "scripts/$hook_file" "$config_dir/$hook_file" 2>/dev/null || return 1
                chmod 644 "$config_dir/$hook_file" 2>/dev/null || return 1
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
            if [ ! -f "$config_dir/$hook_file" ]; then
                cp "scripts/$hook_file" "$config_dir/$hook_file" 2>/dev/null || return 1
                chmod 644 "$config_dir/$hook_file" 2>/dev/null || return 1
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
    if make binary-install 2>/dev/null; then
        echo -e " ${GREEN}OK${RESET}"
    else
        echo -e " ${RED}FAILED${RESET}"
        exit 1
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
            log "You can manually source the appropriate hook file from ${YELLOW}~/.config/git-undo/${RESET}"
            exit 1
            ;;
    esac

    # 3) Final message
    log "${GREEN}Installation completed successfully!${RESET}"
    echo -e ""
    echo -e "Please restart your shell or run '${YELLOW}source ~/.${current_shell}rc${RESET}' to activate ${BLUE}git-undo${RESET}"
}

# Run main function
main "$@"
