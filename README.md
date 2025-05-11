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

- Go 1.16 or newer
- Git
- ZSH shell

### Install Steps

1. First, setup git command logging in your `.zshrc` file by adding the contents of the `git-undo-zsh-hook.zsh` file:

  ```zsh
  # Function to log git commands to repository-specific log file
  log_git_commands() {
    local cmd="$1"

    # Check if this is a git command
    if [[ "$cmd" == git* ]]; then
      # Skip logging "git undo" commands to avoid confusion
      if [[ "$cmd" == "git undo"* ]]; then
        return
      fi

      # Get the git directory for the current repository
      local git_dir=$(git rev-parse --git-dir 2>/dev/null)

      # If we're in a git repository
      if [[ $? -eq 0 ]]; then
        # Create the undo-logs directory if it doesn't exist
        local log_dir="${git_dir}/undo-logs"
        mkdir -p "$log_dir"

        # Log the command with timestamp
        echo "$(date '+%Y-%m-%d %H:%M:%S') $cmd" >> "${log_dir}/command-log.txt"
      fi
    fi
  }

  # Add the function to the preexec hook
  autoload -U add-zsh-hook
  add-zsh-hook preexec log_git_commands
  ```

2. Clone this repository:

  ```bash
  git clone https://github.com/yourusername/git-undo.git
  cd git-undo
  ```

3. Build and install the git plugin:

  ```bash
  go build -o git-undo cmd/git-undo/main.go
  chmod +x git-undo
  cp git-undo /usr/local/bin/
  ```

## Usage

After performing a git command that you want to undo, simply run:

```bash
git undo
```

For verbose mode, which shows more information about what's happening:

```bash
git undo --verbose
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

## Limitations

- `git-undo` can only undo the most recent git command
- Some complex operations might not be fully reversible
- The undo operation must be performed before running other git commands

## License

MIT
