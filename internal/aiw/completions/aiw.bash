_aiw_completion() {
  local current previous
  current="${COMP_WORDS[COMP_CWORD]}"
  previous="${COMP_WORDS[COMP_CWORD-1]}"
  COMPREPLY=()

  if (( COMP_CWORD == 1 )); then
    COMPREPLY=( $(compgen -W 'init agents doctor repo change completion help' -- "$current") )
    return
  fi

  case "${COMP_WORDS[1]}" in
    init)
      if (( COMP_CWORD == 2 )); then
        COMPREPLY=( $(compgen -d -W '--skip-skills' -- "$current") )
      else
        COMPREPLY=( $(compgen -W '--skip-skills' -- "$current") )
      fi
      ;;
    agents)
      (( COMP_CWORD == 2 )) && COMPREPLY=( $(compgen -W 'sync' -- "$current") )
      ;;
    completion)
      (( COMP_CWORD == 2 )) && COMPREPLY=( $(compgen -W 'zsh bash' -- "$current") )
      ;;
    repo)
      if (( COMP_CWORD == 2 )); then
        COMPREPLY=( $(compgen -W 'list register scan sync' -- "$current") )
        return
      fi
      case "${COMP_WORDS[2]}" in
        list)
          COMPREPLY=( $(compgen -W '--json' -- "$current") )
          ;;
        scan)
          (( COMP_CWORD == 3 )) && COMPREPLY=( $(compgen -d -- "$current") )
          ;;
        register)
          if [[ "$previous" == --source ]]; then
            COMPREPLY=( $(compgen -d -- "$current") )
          elif (( COMP_CWORD >= 4 )); then
            COMPREPLY=( $(compgen -W '--source' -- "$current") )
          fi
          ;;
      esac
      ;;
    change)
      if (( COMP_CWORD == 2 )); then
        COMPREPLY=( $(compgen -W 'select add-repo remove-repo materialize setup cleanup repair status list path open close' -- "$current") )
        return
      fi
      case "${COMP_WORDS[2]}" in
        select)
          COMPREPLY=( $(compgen -W '--repo' -- "$current") )
          ;;
        status)
          COMPREPLY=( $(compgen -W '--json' -- "$current") )
          ;;
        close)
          COMPREPLY=( $(compgen -W '--allow-unpushed --discard-changes' -- "$current") )
          ;;
      esac
      ;;
  esac
}

complete -F _aiw_completion aiw
