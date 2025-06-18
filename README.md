# git-undo

# **Ctrl+Z for Git commands**

## 1. `git undo`: one simple command to undo (almost) any Git operation:

```bash
git add .
git commit -m "oops, wrong files"
git undo                           # Back to before commit, files still staged
git undo                           # Back to before add, clean working directory
```

## 2. `git back`: undo navigation commands e.g. `git checkout`, `git switch`:

```bash
# Assume we're on main
git switch feature-branch
git back # back to main
git back # back to feature-branch
```

## 3. `git undo undo` Undoed accidently? Undo it as well. (like Ctrl+Shift+Z)

```bash
git add .
git commit -m "oops, wrong files"
git undo                           # Back to before commit, files still staged
git undo undo                      # Back to commited again
```

## 4. `git undo --dry-run`: see what would be undone:

```bash
# Make some changes
git add file.txt
git undo --dry-run # shows hint to run "git restore --staged ."
git commit -m "test commit"
git undo --dry-run # shows hint to run "git reset --soft HEAD~1"
```

## 5. Debug options: `git undo --verbose`, `git undo --log`

Now you can use Git confidently, knowing any command is easily undoable.

## Installation Options

### One-liner (Recommended)
```bash
curl -fsSL https://raw.githubusercontent.com/amberpixels/git-undo/main/install.sh | bash
```
### Manual Installation (useful for development, debugging, troubleshooting)
```bash
git clone https://github.com/amberpixels/git-undo.git
cd git-undo
./install.sh
```

**Requirements:** Git, Go ≥ 1.21, Bash/Zsh

## Supported commands to be undo-ed:

| Git Command | How it's undone | Notes |
|-------------|-----------------|-------|
| **`git add`** | `git restore --staged <files>` or `git reset <files>` | Unstages files. Uses `git reset` if no HEAD exists |
| **`git commit`** | `git reset --soft HEAD~1` | Keeps changes staged. Handles merge commits and tagged commits |
| **`git branch <name>`** | `git branch -D <name>` | Deletes the created branch |
| **`git checkout -b <name>`** | `git branch -D <name>` | Deletes branch created by checkout -b |
| **`git switch -c <name>`** | `git branch -D <name>` | Deletes branch created by switch -c |
| **`git switch <branch>`** | `git switch -` | Returns to previous branch |
| **`git merge <branch>`** | `git reset --merge ORIG_HEAD` | Handles both fast-forward and merge commits |
| **`git cherry-pick <commit>`** | `git reset --hard HEAD~1` | Removes cherry-picked commit |
| **`git revert <commit>`** | `git reset --hard HEAD~1` | Removes revert commit |
| **`git reset`** | `git reset <previous-head>` | Restores to previous HEAD position using reflog |
| **`git stash` / `git stash push`** | `git stash pop` | Pops and removes the stash |
| **`git rm <files>`** | `git restore --source=HEAD --staged --worktree <files>` | Restores removed files |
| **`git rm --cached <files>`** | `git add <files>` | Re-adds files to index |
| **`git mv <old> <new>`** | `git mv <new> <old>` | Reverses the move operation |
| **`git tag <name>`** | `git tag -d <name>` | Deletes the created tag |
| **`git restore --staged <files>`** | `git add <files>` | Re-stages the files |

### Not Yet Supported (Returns helpful error message):

| Git Command | Reason |
|-------------|--------|
| **`git checkout <branch>`** | Only `checkout -b` is supported (regular checkout navigation not undoable) |
| **`git clean`** | Cannot recover deleted untracked files (would need pre-operation backup) |
| **`git restore --worktree`** | Previous working tree state unknown |
| **`git restore --source=<ref>`** | Previous state from specific reference unknown |
| **`git stash pop/apply`** | Would need to re-stash, which is complex |
| **Branch/tag deletion** | Cannot restore deleted branches/tags (would need backup) |

## How It Works

After installation both `shell hooks` and `git hooks` are installed, that track any git command and send them to `git-undo` (a git plugin) binary. There git commands are categorized and stored in a tiny log file (`.git/git-undo/commands`). Later, when calling `git undo` it reads the log and decide if it's possible (and how) to undo previous command.

## Examples

**Undo a merge:**
```bash
git merge feature-branch
git undo                 # resets --merge ORIG_HEAD
```

**Undo adding specific files:**
```bash
git add file1.js file2.js
git undo                 # unstages just those files
```

**Undo branch creation:**
```bash
git checkout -b new-feature
git undo                 # deletes branch, returns to previous
```

### Self-Management

Get the version information:
```bash
go install github.com/amberpixels/git-undo/cmd/git-undo@latest
```

Update to latest version:
```bash
git undo self update
```

Uninstall:
```bash
git undo self uninstall
```

## Contributing

Found a Git command that should be undoable? [Open an issue](https://github.com/amberpixels/git-undo/issues) or submit a PR!

## License

MIT - see [LICENSE](LICENSE) file.

---

**Make Git worry-free.** [⭐ Star this repo](https://github.com/amberpixels/git-undo) if git-undo makes your development workflow better!
