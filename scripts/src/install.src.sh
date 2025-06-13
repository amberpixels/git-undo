#!/usr/bin/env bash
set -e

# shellcheck disable=SC1091
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
        echo -e "${GRAY}git-undo:${NC} 1. Installing Go binary... ${RED}FAILED${NC} Go not found. ${RED}Go 1.22+ is required to build the binary.${NC}"
        skip_binary=true
    else
        # Extract major & minor (works for go1.xx and goX.YY)
        local ver_raw ver_major ver_minor
        ver_raw=$(go version | awk '{print $3}')       # e.g. go1.22.1
        ver_major=$(printf '%s\n' "$ver_raw" | sed -E 's/go([0-9]+).*/\1/')
        ver_minor=$(printf '%s\n' "$ver_raw" | sed -E 's/go[0-9]+\.([0-9]+).*/\1/')
        detected_go="$ver_raw"

        if  (( ver_major < 1 )) || { (( ver_major == 1 )) && (( ver_minor < 22 )); }; then
            echo -e "${GRAY}git-undo:${NC} 1. Installing Go binary... ${RED}FAILED${NC} Detected Go ${YELLOW}${ver_raw}${NC}, but Go ${RED}≥ 1.22${NC} is required."
            skip_binary=true
        fi
    fi

    if ! $skip_binary; then
         # 1) Install the binary
         echo -en "${GRAY}git-undo:${NC} 1. Installing Go binary (${BLUE}${detected_go}${NC}) ..."

         # Check if we're in dev mode with local source available
         if [[ "${GIT_UNDO_DEV_MODE:-}" == "true" && -d "./cmd/git-undo" && -f "./Makefile" ]]; then
             echo -e " ${YELLOW}(dev mode)${NC}"
             log "Building from local source using Makefile..."

             # Use Makefile's binary-install target which has proper version logic
             if make binary-install &>/dev/null; then
                 # Get the version that was just installed
                 INSTALLED_VERSION=$(git-undo --version 2>/dev/null  || echo "unknown")
                 echo -e "${GRAY}git-undo:${NC} Binary installed with version: ${BLUE}$INSTALLED_VERSION${NC}"
             else
                 echo -e "${GRAY}git-undo:${NC} ${RED}Failed to build from source using Makefile${NC}"
                 exit 1
             fi
         else
             # Normal user installation from GitHub
             if go install "$GITHUB_REPO_URL/cmd/$BIN_NAME@latest" 2>/dev/null; then
                   BIN_PATH=$(command -v git-undo || echo "$BIN_DIR/$BIN_NAME")
                   INSTALLED_VERSION=$(git-undo --version 2>/dev/null  || echo "unknown")
                   echo -e " ${GREEN}OK${NC} (installed at ${BLUE}${BIN_PATH}${NC} | version=${BLUE}${INSTALLED_VERSION}${NC})"
             else
                 echo -e " ${RED}FAILED${NC}"
                 exit 1
             fi
         fi
    fi

    # 2) Git hooks integration
    echo -en "${GRAY}git-undo:${NC} 2. Git integration..."

    current_hooks_path=$(git config --global --get core.hooksPath || echo "")
    target_hooks_path="$GIT_HOOKS_DIR"

    if [[ -z "$current_hooks_path" ]]; then
        git config --global core.hooksPath "$target_hooks_path"
        install_dispatcher_into "$target_hooks_path"
        echo -e " ${GREEN}OK${NC} (set core.hooksPath)"
    elif [[ "$current_hooks_path" == "$target_hooks_path" ]]; then
        install_dispatcher_into "$target_hooks_path"
        echo -e " ${YELLOW}SKIP${NC} (already configured)"
    else
        install_dispatcher_into "$current_hooks_path"
        echo -e " ${YELLOW}SHARED${NC} (pig-backed on $current_hooks_path)"
    fi

    # 3) Shell integration
    local current_shell
    current_shell=$(detect_shell)
    echo -en "${GRAY}git-undo:${NC} 3. Shell integration (${BLUE}$current_shell${NC})..."

    # Temporarily disable set -e to capture non-zero exit codes
    set +e
    local hook_output
    # shellcheck disable=SC2034
    hook_output=$(install_shell_hook "$current_shell" 2>&1)
    local hook_status=$?
    set -e

    case $hook_status in
        0)
            echo -e " ${GREEN}OK${NC}"
            ;;
        2)
            echo -e " ${YELLOW}SKIP${NC} (already configured)"
            ;;
        *)
            echo -e " ${RED}FAILED${NC}"
            log "You can manually source the appropriate hook file from ${YELLOW}$CFG_DIR${NC}"
            exit 1
            ;;
    esac

    # 3) Final message
    log "${GREEN}Installation completed successfully!${NC}"
    echo -e ""
    echo -e "Please restart your shell or run '${YELLOW}source ~/.${current_shell}rc${NC}' to activate ${BLUE}git-undo${NC}"
}

main "$@"
