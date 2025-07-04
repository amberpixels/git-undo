#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
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

    # 1) Remove binaries
    echo -en "${GRAY}git-undo:${NC} 1. Removing binaries..."
    local removed_count=0
    
    if [[ -f "$UNDO_BIN_PATH" ]]; then
        rm -f "$UNDO_BIN_PATH"
        ((removed_count++))
    fi
    
    if [[ -f "$BACK_BIN_PATH" ]]; then
        rm -f "$BACK_BIN_PATH"
        ((removed_count++))
    fi
    
    if [ $removed_count -gt 0 ]; then
        echo -e " ${GREEN}OK${NC} ($removed_count binaries removed)"
    else
        echo -e " ${YELLOW}SKIP${NC} (not found)"
    fi

    # 2) Clean shell configuration files
    echo -en "${GRAY}git-undo:${NC} 2. Cleaning shell configurations..."
    local cleaned_files=0

    # Check each rc file and count successful cleanings
    if scrub_rc "$HOME/.zshrc"; then
        ((cleaned_files++))
    fi
    if scrub_rc "$HOME/.bashrc"; then
        ((cleaned_files++))
    fi
    if scrub_rc "$HOME/.bash_profile"; then
        ((cleaned_files++))
    fi

    if [ $cleaned_files -gt 0 ]; then
        echo -e " ${GREEN}OK${NC} ($cleaned_files files)"
    else
        echo -e " ${YELLOW}SKIP${NC} (no hook lines found)"
    fi

    # 3) Remove config directory
    echo -en "${GRAY}git-undo:${NC} 3. Removing config directory..."
    if [[ -d "$CFG_DIR" ]]; then
        rm -rf "$CFG_DIR"
        echo -e " ${GREEN}OK${NC}"
    else
        echo -e " ${YELLOW}SKIP${NC} (not found)"
    fi

    # 4) Git hooks
    echo -en "${GRAY}git-undo:${NC} 4. Cleaning git hooks…"
    if [[ "$(git config --global --get core.hooksPath)" == "$GIT_HOOKS_DIR" ]]; then
        git config --global --unset core.hooksPath
    fi

    for h in post-commit post-merge; do
        for dir in "$GIT_HOOKS_DIR" "$(git config --global --get core.hooksPath 2>/dev/null || true)"; do
            [[ -z "$dir" ]] && continue
            rm -f "$dir/$h"
        done
    done
    rm -f "$DISPATCHER_FILE"
    echo -e " ${GREEN}OK${NC}"

    # 5) Final message
    log "${GREEN}Uninstallation completed successfully!${NC}"
}

main "$@"
