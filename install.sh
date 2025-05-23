#!/usr/bin/env bash
set -e

GRAY='\033[90m'; GREEN='\033[32m'; YELLOW='\033[33m'; RED='\033[31m'; BLUE='\033[34m'; RESET='\033[0m'
log()  { echo -e "${GRAY}git-undo ↩️:${RESET} $1"; }

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

install_shell_hook() {
    local shell_type="$1"
    local config_dir="$HOME/.config/git-undo"

    # Create config directory with proper permissions
    if [ ! -d "$config_dir" ]; then
        mkdir -p "$config_dir"
        chmod 755 "$config_dir"
    fi

    case "$shell_type" in
        "zsh")
            local hook_file="git-undo-hook.zsh"
            local rc_file="$HOME/.zshrc"
            local source_line="source ~/.config/git-undo/$hook_file"

            # Copy the hook file and set permissions
            cp "scripts/$hook_file" "$config_dir/$hook_file"
            chmod 644 "$config_dir/$hook_file"

            # Add source line to .zshrc if not already present
            if ! grep -qxF "$source_line" "$rc_file" 2>/dev/null; then
                echo "$source_line" >> "$rc_file"
                log "Added '$source_line' to $rc_file"
            else
                log "Hook already configured in $rc_file"
            fi
            ;;

        "bash")
            local hook_file="git-undo-hook.bash"
            local source_line="source ~/.config/git-undo/$hook_file"

            # Copy the hook file and set permissions
            cp "scripts/$hook_file" "$config_dir/$hook_file"
            chmod 644 "$config_dir/$hook_file"

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
                echo "$source_line" >> "$rc_file"
                log "Added '$source_line' to $rc_file"
            else
                log "Hook already configured in $rc_file"
            fi
            ;;

        *)
            log "Warning: Unsupported shell '$shell_type'. Skipping shell integration."
            log "Currently supported shells: zsh, bash"
            return 1
            ;;
    esac

    return 0
}

main() {
    log "Starting installation..."

    echo -en "${GRAY}git-undo ↩️:${RESET} 1. Installing Go binary..."
    if make binary-install 2>/dev/null; then
        echo -e " ${GREEN}OK${RESET}"
    else
        echo -e " ${RED}FAILED${RESET}"
        exit 1
    fi

    local current_shell
    current_shell=$(detect_shell)
    log "2. Shell integration. Shell detected as ${BLUE}$current_shell${RESET}"

    # 3) Install appropriate shell hook
    if install_shell_hook "$current_shell"; then
        log "${GREEN}Installation completed successfully!${RESET}"
        log "Please restart your shell or run '${YELLOW}source ~/.${current_shell}rc${RESET}' to activate ${BLUE}git-undo${RESET}"

        #TODO: restart shell (bash/zsh)
    else
        log "${RED}Shell integration failed.${RESET} You can manually source the appropriate hook file from ${YELLOW}~/.config/git-undo/${RESET}"
        exit 1
    fi
}

# Run main function
main "$@"
