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
