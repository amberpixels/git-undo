#!/usr/bin/env bash
set -e

# 1) Install the git-undo binary
make binary-install

# 2) Copy the hook into ~/.config/git-undo/
mkdir -p ~/.config/git-undo
cp scripts/git-undo-hook.zsh ~/.config/git-undo/git-undo-hook.zsh

# 3) Ensure your ~/.zshrc sources it:
if ! grep -qxF 'source ~/.config/git-undo/git-undo-hook.zsh' ~/.zshrc; then
  echo 'source ~/.config/git-undo/git-undo-hook.zsh' >> ~/.zshrc
  echo 'Added `source ~/.config/git-undo/git-undo-hook.zsh` to ~/.zshrc'
fi

echo "git-undo: Installed! Restarting your shell..."
exec $SHELL -l
