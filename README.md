# git-undo

A Git plugin to undo the previous git command like it was never executed.

## Overview

`git-undo` reads your git command history (stored per repository) and performs the appropriate reverse operation to undo the effects of the last command. It's like a general-purpose "undo" button for git operations.

## Supported Commands

Currently, `git-undo` can revert the following commands:

1. **git commit** - Reverts the commit while keeping the changes staged
2. **git add** - Unstages files that were added
3. **git branch** - Deletes a newly created branch

## Installation

### Prerequisites

- Go 1.18 or newer
- Git
- ZSH shell

### Install

  ```bash
  git clone https://github.com/amberpixels/git-undo.git
  cd git-undo
  ./install.sh
  ```

#### TODOs: easier and better ways to install it

## Usage

After performing a git command that you want to undo, simply run:

```bash
git undo
```

For verbose mode, which shows more information about what's happening:

```bash
git undo --verbose
```

To view the history of git commands that can be undone:

```bash
git undo --log
```

## Examples

1. Undo a commit:

   ```bash
   git commit -m "Some message"
   git undo  # This will undo the commit
   ```

2. Undo adding files:

   ```bash
   git add file1.txt file2.txt
   git undo  # This will unstage file1.txt and file2.txt
   ```

3. Undo creating a branch:

   ```bash
   git branch new-feature
   git undo  # This will delete the new-feature branch
   ```

4. Sequential undo operations:

   ```bash
   git add file1.txt
   git commit -m "First commit"
   git add file2.txt
   git commit -m "Second commit"
   git undo  # This will undo the second commit
   git undo  # This will undo the first commit
   git undo  # This will unstage file1.txt
   ```

## Features

- Undo the most recent git command
- Sequential undo operations (undo multiple commands in reverse order)
- Command history tracking with visual indicators for undoed commands
- Support for common git operations (commit, add, branch)

## Limitations (for now):

- Some complex operations might not be fully reversible

## License

[MIT](LICENSE)
