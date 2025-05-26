#!/usr/bin/env bash
set -e

source "$(dirname "$0")/common.sh"

# Function to write an embedded hook file
write_embedded_hook() {
    local target_file="$1"
    local embedded_var="$2"

    # Decode the base64 embedded content and write it to the target file
    echo "${!embedded_var}" | base64 -d > "$target_file" 2>/dev/null || return 1
    chmod 644 "$target_file" 2>/dev/null || return 1
    return 0
}

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
            local rc_file="$HOME/.zshrc"
            local source_line="source ~/.config/git-undo/git-undo-hook.zsh"

            # Write the embedded hook file
            if [ ! -f "$ZSH_HOOK" ]; then
                write_embedded_hook "$ZSH_HOOK" "EMBEDDED_ZSH_HOOK" || return 1
                is_noop=false
            fi

            # Add source line to .zshrc if not already present
            if ! grep -qxF "$source_line" "$rc_file" 2>/dev/null; then
                echo "$source_line" >> "$rc_file" 2>/dev/null || return 1
                is_noop=false
            fi
            ;;

        "bash")
            local source_line="source ~/.config/git-undo/git-undo-hook.bash"

            # Determine which embedded hook to use
            local embedded_var="EMBEDDED_BASH_HOOK"
            if [[ "${GIT_UNDO_TEST_MODE:-}" == "true" ]]; then
                embedded_var="EMBEDDED_BASH_TEST_HOOK"
            fi

            # Write the embedded hook file
            if [ ! -f "$BASH_HOOK" ]; then
                write_embedded_hook "$BASH_HOOK" "$embedded_var" || return 1
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

    local skip_binary=false
    local detected_go="go-unknown"

    if ! command -v go >/dev/null 2>&1; then
        echo -e "${GRAY}git-undo:${RESET} 1. Installing Go binary... ${RED}FAILED${RESET} Go not found. ${RED}Go 1.22+ is required to build the binary.${RESET}"
        skip_binary=true
    else
        # Extract major & minor (works for go1.xx and goX.YY)
        local ver_raw ver_major ver_minor
        ver_raw=$(go version | awk '{print $3}')       # e.g. go1.22.1
        ver_major=$(printf '%s\n' "$ver_raw" | sed -E 's/go([0-9]+).*/\1/')
        ver_minor=$(printf '%s\n' "$ver_raw" | sed -E 's/go[0-9]+\.([0-9]+).*/\1/')
        detected_go="$ver_raw"

        if  (( ver_major < 1 )) || { (( ver_major == 1 )) && (( ver_minor < 22 )); }; then
            echo -e "${GRAY}git-undo:${RESET} 1. Installing Go binary... ${RED}FAILED${RESET} Detected Go ${YELLOW}${ver_raw}${RESET}, but Go ${RED}≥ 1.22${RESET} is required."
            skip_binary=true
        fi
    fi

    if ! $skip_binary; then
         # 1) Install the binary
         echo -en "${GRAY}git-undo:${RESET} 1. Installing Go binary (${BLUE}${detected_go}${RESET}) ..."

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
             if go install "$GITHUB_REPO_URL/cmd/$BIN_NAME@latest" 2>/dev/null; then
                   BIN_PATH=$(command -v git-undo || echo "$BIN_DIR/$BIN_NAME")
                   echo -e " ${GREEN}OK${RESET} (installed as ${BLUE}${BIN_PATH}${RESET})"
             else
                 echo -e " ${RED}FAILED${RESET}"
                 exit 1
             fi
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
