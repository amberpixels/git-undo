#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "$0")/common.sh"

scrub_rc() {
    local rc="$1"
    [[ -e "$rc" ]] || return 1
    local real_rc="$rc"
    [[ -L "$rc" ]] && real_rc="$(readlink -f "$rc")"

    [[ -f "$real_rc" ]] || return 1

    # Check if hook line exists before attempting to remove it
    if ! grep -q "source .*git-undo-hook" "$real_rc" 2>/dev/null; then
        return 1  # No hook line found, nothing to do
    fi

    # Create backup only if we're going to modify the file
    cp "$real_rc" "${real_rc}.bak.$(date +%s)"

    # cross-platform sed in-place
    if sed --version &>/dev/null; then                    # GNU
        sed -i "/source .*git-undo-hook/d" "$real_rc"
    else                                                  # BSD / macOS
        sed -i '' "/source .*git-undo-hook/d" "$real_rc"
    fi
    return 0  # Successfully cleaned
}

main() {
    log "Starting uninstallation..."

    # 1) Remove binary
    echo -en "${GRAY}git-undo:${RESET} 1. Removing binary..."
    if [[ -f "$BIN_PATH" ]]; then
        rm -f "$BIN_PATH"
        echo -e " ${GREEN}OK${RESET}"
    else
        echo -e " ${YELLOW}SKIP${RESET} (not found)"
    fi

    # 2) Clean shell configuration files
    echo -en "${GRAY}git-undo:${RESET} 2. Cleaning shell configurations..."
    local cleaned_files=0
    
    # Check each rc file and count successful cleanings
    scrub_rc "$HOME/.zshrc" && ((cleaned_files++)) || true
    scrub_rc "$HOME/.bashrc" && ((cleaned_files++)) || true
    scrub_rc "$HOME/.bash_profile" && ((cleaned_files++)) || true
    
    if [ $cleaned_files -gt 0 ]; then
        echo -e " ${GREEN}OK${RESET} ($cleaned_files files)"
    else
        echo -e " ${YELLOW}SKIP${RESET} (no hook lines found)"
    fi

    # 3) Remove config directory
    echo -en "${GRAY}git-undo:${RESET} 3. Removing config directory..."
    if [[ -d "$CFG_DIR" ]]; then
        rm -rf "$CFG_DIR"
        echo -e " ${GREEN}OK${RESET}"
    else
        echo -e " ${YELLOW}SKIP${RESET} (not found)"
    fi

    # 4) Final message
    log "${GREEN}Uninstallation completed successfully!${RESET}"
}

main "$@"
