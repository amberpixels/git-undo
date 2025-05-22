# git-undo ⏪✨

*A universal “Ctrl + Z” for Git commands.* 🔄

`git-undo` tracks every mutating Git command you run and can roll it back with a single `git undo` 🚀  
No reflog spelunking, no cherry‑picks—just instant reversal. ⚡

## Table of Contents
1. [Introduction](#introduction)
2. [Features](#features)
3. [Installation](#installation)
   - [cURL one‑liner](#curl-one-liner-preferred)
   - [Manual clone](#manual-clone)
   - [Shell‑hook integration](#shell-hook-integration)
4. [Quick Start](#quick-start)
5. [Usage](#usage)
6. [Supported Git Commands](#supported-git-commands)
7. [Examples](#examples)
8. [Development & Testing](#development--testing)
9. [Contributing & Feedback](#contributing--feedback)
10. [License](#license)

---

## Introduction
`git-undo` makes destructive Git operations painless to reverse.  
It records each mutating command (commit, add, merge, stash, …) per‑repository in a tiny log file inside `.git/git-undo/`, 
then generates the matching *anti‑command* when you call **`git undo`**.

## Features
- **One‑shot undo** for commits, adds, branches, stashes, merges, and more.
- **Sequential undo / redo** - walk backward *or* forward through history.
- Verbose & dry‑run modes for full transparency.
- Per‑repository command log you can inspect or clear at any time.

## Installation

### Prerequisites
* Git
* Go ≥ 1.21 (auto‑upgrades to 1.24 via Go toolchain)
* ZSH shell (other shells coming soon)

### cURL one‑liner *(preferred)*

```bash
curl -fsSL https://raw.githubusercontent.com/amberpixels/git-undo/main/install.sh | bash
```

### Manual clone
```bash
git clone https://github.com/amberpixels/git-undo.git
cd git-undo
./install.sh
```

### Shell‑hook integration
The installer drops [`scripts/git-undo-hook.zsh`](scripts/git-undo-hook.zsh) into `~/.config/git-undo/`
and appends a `source` line to your `.zshrc`, so every successful Git command is logged automatically.

## Quick Start
```bash
git add .
git commit -m "oops"  # commit, then regret it
git undo              # resets to HEAD~1, keeps changes staged (like Ctrl+Z)
git undo              # undoes `git add .` as well
```

Need the commit back?
```bash
git undo undo      # redo last undo (like Ctrl+Shift+Z)
```

## Usage
| Command              | Effect                                            |
|----------------------|---------------------------------------------------|
| `git undo`           | Roll back the last mutating Git command           |
| `git undo undo`      | Re-roll back the last undoed command              |
| `git undo --verbose` | Show the generated inverse command before running |
| `git undo --dry-run` | Print what *would* be executed, do nothing        |
| `git undo --log`     | Dump your logged command history                  |


## Supported Git Commands
* `commit`
* `add`
* `branch`
* `stash push`
* `merge`
* `checkout -b`
* More on the way—PRs welcome!

## Examples
Undo a merge commit:
```bash
git merge feature/main
git undo          # resets --merge ORIG_HEAD
```

Undo adding specific files:
```bash
git add file1.go file2.go
git undo          # unstages file1.go file2.go
```

## Development & Testing
```bash
make tidy      # fmt, vet, mod tidy
make test      # unit tests
make lint      # golangci‑lint
make build     # compile to ./build/git-undo
make install   # installs Go binary and adds zsh hook
```
## Contributing & Feedback
Spotted a bug or missing undo case?  
Opening an issue or PR makes the tool better for everyone.  
If `git-undo` saved your bacon, please **star the repo** and share suggestions!

## License
[MIT](LICENSE)
