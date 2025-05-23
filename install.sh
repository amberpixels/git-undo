#!/usr/bin/env bash
set -e

# ── colours & logger ───────────────────────────────────────────────────────────
GRAY='\033[90m'; GREEN='\033[32m'; RESET='\033[0m'
log()  { echo -e "${GRAY}git-undo ↩️:${RESET} $1"; }

# Function to detect current shell
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

# Main installation process
main() {
    log "Starting installation..."

    # 1) Install the git-undo binary
    log "Installing Go binary..."
    make binary-install

    # 2) Detect current shell
    local current_shell
    current_shell=$(detect_shell)
    log "Shell integration. Shell detected as $current_shell"

    # 3) Install appropriate shell hook
    if install_shell_hook "$current_shell"; then
        log "${GREEN}Installation completed successfully!${RESET}"
        log "Please restart your shell or run 'source ~/.${current_shell}rc' to activate git-undo"

        #TODO: restart shell (bash/zsh)
    else
        log "Binary installed, but shell integration failed."
        log "You can manually source the appropriate hook file from ~/.config/git-undo/"
        exit 1
    fi
}

# Run main function
main "$@"
