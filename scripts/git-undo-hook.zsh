log_git_command_to_binary() {
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

  GIT_UNDO_INTERNAL_HOOK=1 command git-undo --hook="$raw_cmd"
}

autoload -U add-zsh-hook
add-zsh-hook preexec log_git_command_to_binary
