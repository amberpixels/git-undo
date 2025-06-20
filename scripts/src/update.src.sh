#!/usr/bin/env bash
set -e

# shellcheck disable=SC1091
source "$(dirname "$0")/common.sh"

main() {
    log "Checking for updates..."

    # 1) Get current version from the binary itself
    echo -en "${GRAY}git-undo:${NC} 1. Current version..."
    local current_version
    if ! current_version=$(git-undo version 2>/dev/null | awk '{print $2}'); then
        echo -e " ${RED}FAILED${NC}"
        log "Could not determine current version. Is git-undo installed?"
        exit 1
    fi

    if [[ -z "$current_version" || "$current_version" == "unknown" ]]; then
        echo -e " ${YELLOW}UNKNOWN${NC}"
        log_warning "No version information found. Reinstall git-undo manually."
        exit 1
    else
        echo -e " ${BLUE}$current_version${NC}"
    fi

    # 2) Get latest release version
    echo -en "${GRAY}git-undo:${NC} 2. Checking latest release..."
    local latest_version
    if ! latest_version=$(get_latest_version); then
        echo -e " ${RED}FAILED${NC}"
        log "Failed to check latest version. Check your internet connection."
        exit 1
    fi
    echo -e " ${BLUE}$latest_version${NC}"

    # 3) Compare versions
    echo -en "${GRAY}git-undo:${NC} 3. Comparing releases..."
    local comparison
    comparison=$(version_compare "$current_version" "$latest_version")

    case "$comparison" in
        "same")
            echo -e " ${GREEN}UP TO DATE${NC}"
            log "You're already running the latest release (${BLUE}$current_version${NC})"
            exit 0
            ;;
        "newer")
            echo -e " ${YELLOW}NEWER${NC}"
            log "You're running a newer release than available (${BLUE}$current_version${NC} > ${BLUE}$latest_version${NC})"
            exit 0
            ;;
        "older")
            echo -e " ${YELLOW}UPDATE AVAILABLE${NC}"
            ;;
    esac

    # 4) Ask for confirmation
    echo -e ""
    echo -e "Update available: ${BLUE}$current_version${NC} → ${GREEN}$latest_version${NC}"
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
    echo -en "${GRAY}git-undo:${NC} 4. Downloading latest installer..."
    local temp_installer
    temp_installer=$(mktemp)

    if command -v curl >/dev/null 2>&1; then
        if curl -sL "$INSTALL_URL" -o "$temp_installer"; then
            echo -e " ${GREEN}OK${NC}"
        else
            echo -e " ${RED}FAILED${NC}"
            rm -f "$temp_installer"
            exit 1
        fi
    elif command -v wget >/dev/null 2>&1; then
        if wget -qO "$temp_installer" "$INSTALL_URL"; then
            echo -e " ${GREEN}OK${NC}"
        else
            echo -e " ${RED}FAILED${NC}"
            rm -f "$temp_installer"
            exit 1
        fi
    else
        echo -e " ${RED}FAILED${NC}"
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
        log "${GREEN}Update completed successfully!${NC}"
        log "Updated to version ${GREEN}$latest_version${NC}"
    else
        log "${RED}Update failed.${NC}"
        exit 1
    fi
}

# Run main function
main "$@"
