#!/usr/bin/env bash
set -e

# Color codes
GRAY='\033[90m'
GREEN='\033[32m'
RESET='\033[0m'

# Function to print with git-undo prefix in gray
say() {
    local message="$1"
    local color="$2"
    if [[ -n "$color" ]]; then
        echo -e "${GRAY}git-undo ↩️:${RESET} ${color}$message${RESET}"
    else
        echo -e "${GRAY}git-undo ↩️:${RESET} $message"
    fi
}

# Function to detect current shell
detect_shell() {
    local shell_name

    # Method 1: Check $0 (most reliable for current shell)
    if [[ "$0" == *"bash"* ]]; then
        echo "bash"
        return
    elif [[ "$0" == *"zsh"* ]]; then
        echo "zsh"
        return
    fi

    # Method 2: Check current process name
    shell_name=$(ps -p $ -o comm= 2>/dev/null | tr -d '[:space:]')
    case "$shell_name" in
        *zsh*)
            echo "zsh"
            return
            ;;
        *bash*)
            echo "bash"
            return
            ;;
    esac

    # Method 3: Check BASH_VERSION or ZSH_VERSION environment variables
    if [[ -n "$BASH_VERSION" ]]; then
        echo "bash"
        return
    elif [[ -n "$ZSH_VERSION" ]]; then
        echo "zsh"
        return
    fi

    # Method 4: Fallback to $SHELL environment variable
    if [[ -n "$SHELL" ]]; then
        shell_name=$(basename "$SHELL")
        case "$shell_name" in
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

    # If all methods fail
    echo "unknown"
}

# Function to install shell hook
install_shell_hook() {
    local shell_type="$1"
    local config_dir="$HOME/.config/git-undo"

    # Create config directory
    mkdir -p "$config_dir"

    case "$shell_type" in
        "zsh")
            local hook_file="git-undo-hook.zsh"
            local rc_file="$HOME/.zshrc"
            local source_line="source ~/.config/git-undo/$hook_file"

            # Copy the hook file
            cp "scripts/$hook_file" "$config_dir/$hook_file"

            # Add source line to .zshrc if not already present
            if ! grep -qxF "$source_line" "$rc_file" 2>/dev/null; then
                echo "$source_line" >> "$rc_file"
                say "Added '$source_line' to $rc_file"
            else
                say "Hook already configured in $rc_file"
            fi
            ;;

        "bash")
            local hook_file="git-undo-hook.bash"
            local source_line="source ~/.config/git-undo/$hook_file"

            # Copy the hook file
            cp "scripts/$hook_file" "$config_dir/$hook_file"

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
                say "Added '$source_line' to $rc_file"
            else
                say "Hook already configured in $rc_file"
            fi
            ;;

        *)
            say "Warning: Unsupported shell '$shell_type'. Skipping shell integration."
            say "Currently supported shells: zsh, bash"
            return 1
            ;;
    esac

    return 0
}

# Main installation process
main() {
    say "Starting installation..."

    # 1) Install the git-undo binary
    say "Installing Go binary..."
    make binary-install

    # 2) Detect current shell
    local current_shell
    current_shell=$(detect_shell)
    say "Shell integration. Shell detected as $current_shell"

    # 3) Install appropriate shell hook
    if install_shell_hook "$current_shell"; then
        say "Installation completed successfully!" "$GREEN"
        say "Please restart your shell or run 'source ~/.${current_shell}rc' to activate git-undo"

        #TODO: restart shell (bash/zsh)
    else
        say "Binary installed, but shell integration failed."
        say "You can manually source the appropriate hook file from ~/.config/git-undo/"
        exit 1
    fi
}

# Run main function
main "$@"
