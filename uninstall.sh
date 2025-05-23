#!/usr/bin/env bash
set -euo pipefail

GRAY='\033[90m'; GREEN='\033[32m'; YELLOW='\033[33m'; RED='\033[31m'; BLUE='\033[34m'; RESET='\033[0m'
log()  { echo -e "${GRAY}git-undo:${RESET} $1"; }

BIN_NAME="git-undo"
BIN_DIR=$(go env GOBIN 2>/dev/null || true)
[[ -z "$BIN_DIR" ]] && BIN_DIR="$(go env GOPATH)/bin"
BIN_PATH="$BIN_DIR/$BIN_NAME"

CFG_DIR="$HOME/.config/git-undo"
ZSH_HOOK="$CFG_DIR/git-undo-hook.zsh"
BASH_HOOK="$CFG_DIR/git-undo-hook.bash"

if [[ -f "$BIN_PATH" ]]; then
  rm -f "$BIN_PATH"
  log "1. Removed binary at ${YELLOW}$BIN_PATH${RESET}"
else
  log "1. Binary not found at ${YELLOW}$BIN_PATH${RESET} (skipped)"
fi

scrub_rc() {                  # $1 = rc path
  local rc="$1"
  [[ -e "$rc" ]] || return
  local real_rc="$rc"
  [[ -L "$rc" ]] && real_rc="$(readlink -f "$rc")"

  [[ -f "$real_rc" ]] || { log "$rc is not a regular file (skipped)"; return; }

  # Check if hook line exists before attempting to remove it
  if ! grep -q "source .*git-undo-hook" "$real_rc" 2>/dev/null; then
    return  # No hook line found, nothing to do
  fi

  # Create backup only if we're going to modify the file
  cp "$real_rc" "${real_rc}.bak.$(date +%s)"

  # cross-platform sed in-place
  if sed --version &>/dev/null; then                    # GNU
    sed -i "/source .*git-undo-hook/d" "$real_rc"
  else                                                  # BSD / macOS
    sed -i '' "/source .*git-undo-hook/d" "$real_rc"
  fi
  log "2. Cleaned hook line from ${YELLOW}$rc${RESET}"
}
scrub_rc "$HOME/.zshrc"
scrub_rc "$HOME/.bashrc"
scrub_rc "$HOME/.bash_profile"

if [[ -d "$CFG_DIR" ]]; then
  rm -rf "$CFG_DIR"
  log "3. Removed config directory at ${YELLOW}$CFG_DIR${RESET}"
else
  log "3. Config directory not found at ${YELLOW}$CFG_DIR${RESET} (skipped)"
fi

log "${GREEN}git-undo uninstalled successfully.${RESET}"
