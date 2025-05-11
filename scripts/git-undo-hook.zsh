# Function to store the git command temporarily
store_git_command() {
  local raw_cmd="$1"
  local head=${raw_cmd%% *}
  local rest=${raw_cmd#"$head"}
  if alias "$head" &>/dev/null; then
    local def=$(alias "$head")
    local expansion=${def#*\'}
    expansion=${expansion%\'}
    raw_cmd="${expansion}${rest}"
  fi
  [[ "$raw_cmd" == git\ * ]] || return
  GIT_COMMAND_TO_LOG="$raw_cmd"
}

# Function to log the command only if it was successful
log_successful_git_command() {
  # Check if we have a git command to log and if the previous command was successful
  if [[ -n "$GIT_COMMAND_TO_LOG" && $? -eq 0 ]]; then
    GIT_UNDO_INTERNAL_HOOK=1 command git-undo -v --hook="$GIT_COMMAND_TO_LOG"
  fi
  # Clear the stored command
  GIT_COMMAND_TO_LOG=""
}

autoload -U add-zsh-hook
add-zsh-hook preexec store_git_command
add-zsh-hook precmd log_successful_git_command
