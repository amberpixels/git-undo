#!/usr/bin/env bash
set -e

source "$(dirname "$0")/common.sh"

main() {
    log "Checking for updates..."

    # 1) Get current tag version (ignore commits)
    echo -en "${GRAY}git-undo:${RESET} 1. Current version..."
    local current_version
    current_version=$(get_current_tag_version)
    if [[ "$current_version" == "unknown" ]]; then
        echo -e " ${YELLOW}UNKNOWN${RESET}"
        log "No version information found. Run '${YELLOW}git-undo --version${RESET}' or reinstall."
        exit 1
    else
        echo -e " ${BLUE}$current_version${RESET}"
    fi

    # 2) Get latest release version
    echo -en "${GRAY}git-undo:${RESET} 2. Checking latest release..."
    local latest_version
    if ! latest_version=$(get_latest_version); then
        echo -e " ${RED}FAILED${RESET}"
        log "Failed to check latest version. Check your internet connection."
        exit 1
    fi
    echo -e " ${BLUE}$latest_version${RESET}"

    # 3) Compare tag versions only
    echo -en "${GRAY}git-undo:${RESET} 3. Comparing releases..."
    local comparison
    comparison=$(version_compare "$current_version" "$latest_version")
    
    case "$comparison" in
        "same")
            echo -e " ${GREEN}UP TO DATE${RESET}"
            log "You're already running the latest release (${BLUE}$current_version${RESET})"
            exit 0
            ;;
        "newer")
            echo -e " ${YELLOW}NEWER${RESET}"
            log "You're running a newer release than available (${BLUE}$current_version${RESET} > ${BLUE}$latest_version${RESET})"
            exit 0
            ;;
        "older")
            echo -e " ${YELLOW}UPDATE AVAILABLE${RESET}"
            ;;
    esac

    # 4) Ask for confirmation
    echo -e ""
    echo -e "Update available: ${BLUE}$current_version${RESET} → ${GREEN}$latest_version${RESET}"
    echo -en "Do you want to update? [Y/n]: "
    read -r response
    
    case "$response" in
        [nN]|[nN][oO])
            log "Update cancelled."
            exit 0
            ;;
        *)
            ;;
    esac

    # 5) Download and run new installer
    echo -en "${GRAY}git-undo:${RESET} 4. Downloading latest installer..."
    local temp_installer
    temp_installer=$(mktemp)
    
    if command -v curl >/dev/null 2>&1; then
        if curl -sL "$INSTALL_URL" -o "$temp_installer"; then
            echo -e " ${GREEN}OK${RESET}"
        else
            echo -e " ${RED}FAILED${RESET}"
            rm -f "$temp_installer"
            exit 1
        fi
    elif command -v wget >/dev/null 2>&1; then
        if wget -qO "$temp_installer" "$INSTALL_URL"; then
            echo -e " ${GREEN}OK${RESET}"
        else
            echo -e " ${RED}FAILED${RESET}"
            rm -f "$temp_installer"
            exit 1
        fi
    else
        echo -e " ${RED}FAILED${RESET}"
        log "curl or wget required for update"
        exit 1
    fi

    # 6) Run the installer
    echo -e ""
    log "Running installer..."
    chmod +x "$temp_installer"
    "$temp_installer"
    local install_status=$?
    rm -f "$temp_installer"

    if [[ $install_status -eq 0 ]]; then
        log "${GREEN}Update completed successfully!${RESET}"
        log "Updated to version ${GREEN}$latest_version${RESET}"
    else
        log "${RED}Update failed.${RESET}"
        exit 1
    fi
}

# Run main function
main "$@" 