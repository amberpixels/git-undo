# Variable to store the git command temporarily
GIT_COMMAND_TO_LOG=""

# Function to store the git command temporarily
store_git_command() {
  local raw_cmd="$1"
  local head=${raw_cmd%% *}
  local rest=${raw_cmd#"$head"}

  # Check if the command is an alias and expand it
  if alias "$head" &>/dev/null; then
    local def=$(alias "$head")
    # Extract the expansion from alias output (format: alias name='expansion')
    local expansion=${def#*\'}
    expansion=${expansion%\'}
    raw_cmd="${expansion}${rest}"
  fi

  # Only store if it's a git command
  [[ "$raw_cmd" == git\ * ]] || return
  GIT_COMMAND_TO_LOG="$raw_cmd"
}

# Function to log the command only if it was successful
log_successful_git_command() {
  # Check if we have a git command to log and if the previous command was successful
  if [[ -n "$GIT_COMMAND_TO_LOG" && $? -eq 0 ]]; then
    GIT_UNDO_INTERNAL_HOOK=1 command git-undo --hook="$GIT_COMMAND_TO_LOG"
  fi
  # Clear the stored command
  GIT_COMMAND_TO_LOG=""
}


# Test mode: provide a manual way to capture commands
# This is only used for integration-test.bats. 
git() {
    command git "$@"
    local exit_code=$?
    if [[ $exit_code -eq 0 ]]; then
        GIT_UNDO_INTERNAL_HOOK=1 command git-undo --hook="git $*"
    fi
    return $exit_code
}


# Set up PROMPT_COMMAND to log successful commands after execution
if [[ -z "$PROMPT_COMMAND" ]]; then
  PROMPT_COMMAND="log_successful_git_command"
else
  PROMPT_COMMAND="$PROMPT_COMMAND; log_successful_git_command"
fi
