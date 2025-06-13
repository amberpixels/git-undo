#!/usr/bin/env bash
set -e

# Parse command line arguments
VERBOSE=false
while [[ $# -gt 0 ]]; do
    case $1 in
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        *)
            shift
            ;;
    esac
done

# shellcheck disable=SC1091
source "$(dirname "$0")/common.sh"

# Verbose logging function
verbose_log() {
    if $VERBOSE; then
        echo -e "${GRAY}[VERBOSE]${NC} $1"
    fi
}

# Function to write an embedded hook file
write_embedded_hook() {
    local target_file="$1"
    local embedded_var="$2"

    verbose_log "Writing embedded hook file: $target_file"
    # Decode the base64 embedded content and write it to the target file
    if echo "${!embedded_var}" | base64 -d > "$target_file" 2>/dev/null; then
        verbose_log "Successfully wrote hook file: $target_file"
    else
        verbose_log "Failed to write hook file: $target_file"
        return 1
    fi
    
    if chmod 644 "$target_file" 2>/dev/null; then
        verbose_log "Successfully set permissions on: $target_file"
    else
        verbose_log "Failed to set permissions on: $target_file"
        return 1
    fi
    return 0
}

install_shell_hook() {
    local shell_type="$1"
    local is_noop=true

    verbose_log "Installing shell hook for: $shell_type"
    
    # Create config directory with proper permissions
    if [ ! -d "$CFG_DIR" ]; then
        verbose_log "Creating config directory: $CFG_DIR"
        if mkdir -p "$CFG_DIR" 2>/dev/null; then
            verbose_log "Successfully created config directory"
        else
            verbose_log "Failed to create config directory: $CFG_DIR"
            return 1
        fi
        
        if chmod 755 "$CFG_DIR" 2>/dev/null; then
            verbose_log "Successfully set permissions on config directory"
        else
            verbose_log "Failed to set permissions on config directory"
            return 1
        fi
        is_noop=false
    else
        verbose_log "Config directory already exists: $CFG_DIR"
    fi

    case "$shell_type" in
        "zsh")
            local rc_file="$HOME/.zshrc"
            local source_line="source ~/.config/git-undo/git-undo-hook.zsh"

            verbose_log "Processing zsh hook installation"
            verbose_log "RC file: $rc_file"
            verbose_log "Hook file: $ZSH_HOOK"
            
            # Write the embedded hook file
            if [ ! -f "$ZSH_HOOK" ]; then
                verbose_log "Hook file doesn't exist, creating it"
                if write_embedded_hook "$ZSH_HOOK" "EMBEDDED_ZSH_HOOK"; then
                    verbose_log "Successfully created zsh hook file"
                    is_noop=false
                else
                    verbose_log "Failed to create zsh hook file"
                    return 1
                fi
            else
                verbose_log "Hook file already exists: $ZSH_HOOK"
            fi

            # Add source line to .zshrc if not already present
            if ! grep -qxF "$source_line" "$rc_file" 2>/dev/null; then
                verbose_log "Adding source line to $rc_file"
                if echo "$source_line" >> "$rc_file" 2>/dev/null; then
                    verbose_log "Successfully added source line to $rc_file"
                    is_noop=false
                else
                    verbose_log "Failed to add source line to $rc_file"
                    return 1
                fi
            else
                verbose_log "Source line already exists in $rc_file"
            fi
            ;;

        "bash")
            local source_line="source ~/.config/git-undo/git-undo-hook.bash"

            verbose_log "Processing bash hook installation"
            
            # Determine which embedded hook to use
            local embedded_var="EMBEDDED_BASH_HOOK"
            if [[ "${GIT_UNDO_TEST_MODE:-}" == "true" ]]; then
                embedded_var="EMBEDDED_BASH_TEST_HOOK"
                verbose_log "Using test mode bash hook"
            fi
            verbose_log "Using embedded variable: $embedded_var"
            verbose_log "Hook file: $BASH_HOOK"

            # Write the embedded hook file
            if [ ! -f "$BASH_HOOK" ]; then
                verbose_log "Hook file doesn't exist, creating it"
                if write_embedded_hook "$BASH_HOOK" "$embedded_var"; then
                    verbose_log "Successfully created bash hook file"
                    is_noop=false
                else
                    verbose_log "Failed to create bash hook file"
                    return 1
                fi
            else
                verbose_log "Hook file already exists: $BASH_HOOK"
            fi

            # Determine which bash config file to use
            local rc_file
            if [[ "$OSTYPE" == "darwin"* ]]; then
                # macOS uses .bash_profile for login shells (default in Terminal.app)
                rc_file="$HOME/.bash_profile"
                verbose_log "macOS detected, using .bash_profile"
            else
                # Linux typically uses .bashrc for interactive shells
                rc_file="$HOME/.bashrc"
                verbose_log "Linux detected, using .bashrc"
            fi
            verbose_log "RC file: $rc_file"

            # Add source line to the appropriate file if not already present
            if ! grep -qxF "$source_line" "$rc_file" 2>/dev/null; then
                verbose_log "Adding source line to $rc_file"
                if echo "$source_line" >> "$rc_file" 2>/dev/null; then
                    verbose_log "Successfully added source line to $rc_file"
                    is_noop=false
                else
                    verbose_log "Failed to add source line to $rc_file"
                    return 1
                fi
            else
                verbose_log "Source line already exists in $rc_file"
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
    verbose_log "Verbose mode enabled"

    local skip_binary=false
    local detected_go="go-unknown"

    verbose_log "Checking for Go installation..."
    if ! command -v go >/dev/null 2>&1; then
        verbose_log "Go command not found in PATH"
        echo -e "${GRAY}git-undo:${NC} 1. Installing Go binary... ${RED}FAILED${NC} Go not found. ${RED}Go 1.22+ is required to build the binary.${NC}"
        skip_binary=true
    else
        verbose_log "Go found, checking version..."
        # Extract major & minor (works for go1.xx and goX.YY)
        local ver_raw ver_major ver_minor
        ver_raw=$(go version | awk '{print $3}')       # e.g. go1.22.1
        ver_major=$(printf '%s\n' "$ver_raw" | sed -E 's/go([0-9]+).*/\1/')
        ver_minor=$(printf '%s\n' "$ver_raw" | sed -E 's/go[0-9]+\.([0-9]+).*/\1/')
        detected_go="$ver_raw"
        
        verbose_log "Detected Go version: $ver_raw (major: $ver_major, minor: $ver_minor)"

        if  (( ver_major < 1 )) || { (( ver_major == 1 )) && (( ver_minor < 22 )); }; then
            verbose_log "Go version is too old (< 1.22)"
            echo -e "${GRAY}git-undo:${NC} 1. Installing Go binary... ${RED}FAILED${NC} Detected Go ${YELLOW}${ver_raw}${NC}, but Go ${RED}≥ 1.22${NC} is required."
            skip_binary=true
        else
            verbose_log "Go version is acceptable (>= 1.22)"
        fi
    fi

    if ! $skip_binary; then
         verbose_log "Proceeding with binary installation"
         # 1) Install the binaries
         echo -en "${GRAY}git-undo:${NC} 1. Installing Go binaries (${BLUE}${detected_go}${NC}) ..."

         # Check if we're in dev mode with local source available
         if [[ "${GIT_UNDO_DEV_MODE:-}" == "true" && -d "./cmd/git-undo" && -f "./Makefile" ]]; then
             echo -e " ${YELLOW}(dev mode)${NC}"
             verbose_log "Dev mode detected, building from local source"
             log "Building from local source using Makefile..."

             # Use Makefile's binary-install target which has proper version logic
             verbose_log "Running 'make binary-install'..."
             if $VERBOSE; then
                 # Run with output visible in verbose mode
                 if make binary-install; then
                     verbose_log "make binary-install succeeded"
                     # Get the version that was just installed
                     INSTALLED_VERSION=$(git-undo --version 2>/dev/null  || echo "unknown")
                     echo -e "${GRAY}git-undo:${NC} Binaries installed with version: ${BLUE}$INSTALLED_VERSION${NC}"
                     log "Installed: git-undo and git-back"
                 else
                     verbose_log "make binary-install failed"
                     echo -e "${GRAY}git-undo:${NC} ${RED}Failed to build from source using Makefile${NC}"
                     exit 1
                 fi
             else
                 # Run silently in non-verbose mode
                 if make binary-install &>/dev/null; then
                     verbose_log "make binary-install succeeded"
                     # Get the version that was just installed
                     INSTALLED_VERSION=$(git-undo --version 2>/dev/null  || echo "unknown")
                     echo -e "${GRAY}git-undo:${NC} Binaries installed with version: ${BLUE}$INSTALLED_VERSION${NC}"
                     log "Installed: git-undo and git-back"
                 else
                     verbose_log "make binary-install failed"
                     echo -e "${GRAY}git-undo:${NC} ${RED}Failed to build from source using Makefile${NC}"
                     exit 1
                 fi
             fi
         else
             verbose_log "Normal installation mode from GitHub"
             # Normal user installation from GitHub - install both binaries
             local undo_failed=false
             local back_failed=false
             
             verbose_log "Installing git-undo from $GITHUB_REPO_URL/cmd/$UNDO_BIN_NAME@latest"
             # Install git-undo (required)
             if $VERBOSE; then
                 if ! go install "$GITHUB_REPO_URL/cmd/$UNDO_BIN_NAME@latest"; then
                     verbose_log "git-undo installation failed"
                     undo_failed=true
                 else
                     verbose_log "git-undo installation succeeded"
                 fi
             else
                 if ! go install "$GITHUB_REPO_URL/cmd/$UNDO_BIN_NAME@latest" 2>/dev/null; then
                     verbose_log "git-undo installation failed"
                     undo_failed=true
                 else
                     verbose_log "git-undo installation succeeded"
                 fi
             fi
             
             # If git-undo failed, that's a critical error
             if $undo_failed; then
                 verbose_log "git-undo installation failed - this is required"
                 echo -e " ${RED}FAILED${NC} (git-undo installation failed)"
                 exit 1
             fi
             
             verbose_log "Installing git-back from $GITHUB_REPO_URL/cmd/$BACK_BIN_NAME@latest"
             # Install git-back (optional)
             if $VERBOSE; then
                 if ! go install "$GITHUB_REPO_URL/cmd/$BACK_BIN_NAME@latest"; then
                     verbose_log "git-back installation failed - continuing without it"
                     back_failed=true
                 else
                     verbose_log "git-back installation succeeded"
                 fi
             else
                 if ! go install "$GITHUB_REPO_URL/cmd/$BACK_BIN_NAME@latest" 2>/dev/null; then
                     verbose_log "git-back installation failed - continuing without it"
                     back_failed=true
                 else
                     verbose_log "git-back installation succeeded"
                 fi
             fi
             
             # Success message based on what was installed
             UNDO_BIN_PATH=$(command -v git-undo || echo "$BIN_DIR/$UNDO_BIN_NAME")
             INSTALLED_VERSION=$(git-undo --version 2>/dev/null  || echo "unknown")
             verbose_log "git-undo path: $UNDO_BIN_PATH"
             verbose_log "Installed version: $INSTALLED_VERSION"
             
             if $back_failed; then
                 verbose_log "git-back installation failed, but git-undo succeeded"
                 echo -e " ${YELLOW}PARTIAL${NC} (git-undo: ${BLUE}${UNDO_BIN_PATH}${NC} | version=${BLUE}${INSTALLED_VERSION}${NC})"
                 log "${YELLOW}Warning: git-back could not be installed (not available in this version). Only git-undo is available.${NC}"
             else
                 BACK_BIN_PATH=$(command -v git-back || echo "$BIN_DIR/$BACK_BIN_NAME")
                 verbose_log "git-back path: $BACK_BIN_PATH"
                 verbose_log "All binary installations succeeded"
                 echo -e " ${GREEN}OK${NC} (git-undo: ${BLUE}${UNDO_BIN_PATH}${NC}, git-back: ${BLUE}${BACK_BIN_PATH}${NC} | version=${BLUE}${INSTALLED_VERSION}${NC})"
             fi
         fi
    else
        verbose_log "Skipping binary installation due to Go issues"
    fi

    # 2) Git hooks integration
    echo -en "${GRAY}git-undo:${NC} 2. Git integration..."
    verbose_log "Starting git hooks integration"

    current_hooks_path=$(git config --global --get core.hooksPath || echo "")
    target_hooks_path="$GIT_HOOKS_DIR"
    
    verbose_log "Current global hooks path: '${current_hooks_path}'"
    verbose_log "Target hooks path: '${target_hooks_path}'"

    if [[ -z "$current_hooks_path" ]]; then
        verbose_log "No global hooks path set, configuring our hooks path"
        if git config --global core.hooksPath "$target_hooks_path"; then
            verbose_log "Successfully set global core.hooksPath to $target_hooks_path"
        else
            verbose_log "Failed to set global core.hooksPath"
        fi
        
        verbose_log "Installing dispatcher into $target_hooks_path"
        install_dispatcher_into "$target_hooks_path"
        echo -e " ${GREEN}OK${NC} (set core.hooksPath)"
    elif [[ "$current_hooks_path" == "$target_hooks_path" ]]; then
        verbose_log "Hooks path already configured correctly"
        verbose_log "Installing dispatcher into $target_hooks_path"
        install_dispatcher_into "$target_hooks_path"
        echo -e " ${YELLOW}SKIP${NC} (already configured)"
    else
        verbose_log "Different hooks path already configured, piggybacking on it"
        verbose_log "Installing dispatcher into existing path: $current_hooks_path"
        install_dispatcher_into "$current_hooks_path"
        echo -e " ${YELLOW}SHARED${NC} (pig-backed on $current_hooks_path)"
    fi

    # 3) Shell integration
    verbose_log "Starting shell integration"
    local current_shell
    current_shell=$(detect_shell)
    verbose_log "Detected shell: $current_shell"
    echo -en "${GRAY}git-undo:${NC} 3. Shell integration (${BLUE}$current_shell${NC})..."

    # Temporarily disable set -e to capture non-zero exit codes
    set +e
    local hook_output
    # shellcheck disable=SC2034
    if $VERBOSE; then
        verbose_log "Running install_shell_hook with output visible"
        hook_output=$(install_shell_hook "$current_shell" 2>&1)
        local hook_status=$?
        verbose_log "install_shell_hook output: $hook_output"
    else
        hook_output=$(install_shell_hook "$current_shell" 2>&1)
        local hook_status=$?
    fi
    verbose_log "install_shell_hook exit status: $hook_status"
    set -e

    case $hook_status in
        0)
            verbose_log "Shell integration succeeded"
            echo -e " ${GREEN}OK${NC}"
            ;;
        2)
            verbose_log "Shell integration already configured"
            echo -e " ${YELLOW}SKIP${NC} (already configured)"
            ;;
        *)
            verbose_log "Shell integration failed with status $hook_status"
            verbose_log "Hook output: $hook_output"
            echo -e " ${RED}FAILED${NC}"
            log "You can manually source the appropriate hook file from ${YELLOW}$CFG_DIR${NC}"
            exit 1
            ;;
    esac

    # 4) Final message
    verbose_log "Installation process completed"
    log "${GREEN}Installation completed successfully!${NC}"
    echo -e ""
    echo -e "Please restart your shell or run '${YELLOW}source ~/.${current_shell}rc${NC}' to activate ${BLUE}git-undo${NC}"
}

main "$@"
