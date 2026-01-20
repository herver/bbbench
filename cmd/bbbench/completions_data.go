package main

const bashCompletion = `_bbbench() {
  local cur cmds
  cur="${COMP_WORDS[COMP_CWORD]}"
  cmds="generate validate-templates doctor completion help"

  if [[ $COMP_CWORD == 1 ]]; then
    COMPREPLY=( $(compgen -W "$cmds" -- "$cur") )
    return 0
  fi

  # complete completion shells
  if [[ ${COMP_WORDS[1]} == "completion" && $COMP_CWORD == 2 ]]; then
    COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
    return 0
  fi
}
complete -F _bbbench bbbench
`

const zshCompletion = `#compdef bbbench

_arguments \
  '1:cmd:(generate validate-templates doctor completion help)' \
  '2:shell:(bash zsh fish)'
`

const fishCompletion = `complete -c bbbench -f -a "generate validate-templates doctor completion help"
complete -c bbbench -n "__fish_seen_subcommand_from completion" -f -a "bash zsh fish"
`
