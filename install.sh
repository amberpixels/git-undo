#!/usr/bin/env bash
set -e

# 1) Build or download the git-undo binary.
make build  # or: curl -L …/git-undo > ~/.local/bin/git-undo && chmod +x ~/.local/bin/git-undo

# 2) Copy the hook into ~/.config/git-undo/
mkdir -p ~/.config/git-undo
cp scripts/git-undo-hook.zsh ~/.config/git-undo/git-undo-hook.zsh

# 3) Ensure your ~/.zshrc sources it:
if ! grep -qxF 'source ~/.config/git-undo/git-undo-hook.zsh' ~/.zshrc; then
  echo 'source ~/.config/git-undo/git-undo-hook.zsh' >> ~/.zshrc
  echo 'Added `source ~/.config/git-undo/git-undo-hook.zsh` to ~/.zshrc'
fi

echo "Installed git-undo! Restarting your shell..."
exec $SHELL -l
