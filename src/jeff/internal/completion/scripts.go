package completion

import (
	"fmt"
	"strings"
)

// Script renders the completion script for the provided shell.
func Script(shell string) (string, error) {
	switch strings.ToLower(shell) {
	case "bash":
		return bashScript, nil
	case "zsh":
		return zshScript, nil
	case "fish":
		return fishScript, nil
	default:
		return "", fmt.Errorf("unknown shell %q", shell)
	}
}

const bashScript = `# bash completion for jeff
_jeff_completion() {
    local cur
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    if [[ ${COMP_CWORD} -eq 1 ]]; then
        COMPREPLY=( $(compgen -W "codex completion help" -- "$cur") )
        return 0
    fi

    case "${COMP_WORDS[1]}" in
        codex)
            if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "init ask tui" -- "$cur") )
                return 0
            fi
            case "${COMP_WORDS[2]}" in
                init)
                    COMPREPLY=( $(compgen -W "--session --last-session --codex-binary" -- "$cur") )
                    ;;
                ask)
                    COMPREPLY=( $(compgen -W "--session --codex-binary --timeout --show-token-cost" -- "$cur") )
                    ;;
                tui)
                    COMPREPLY=()
                    ;;
                *)
                    ;;
            esac
            ;;
        completion)
            if [[ ${COMP_CWORD} -eq 2 ]]; then
                COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
            else
                COMPREPLY=( $(compgen -W "--dir --print" -- "$cur") )
            fi
            ;;
        *)
            ;;
    esac
}
complete -F _jeff_completion jeff
`

const zshScript = `#compdef jeff

_jeff() {
  local -a commands
  commands=(
    'codex:Codex integration commands'
    'completion:Generate completions'
    'help:Show help'
  )

  if (( CURRENT == 2 )); then
    _describe 'command' commands
    return
  fi

  case "$words[2]" in
    codex)
      if (( CURRENT == 3 )); then
        _values 'codex command' init ask tui
        return
      fi
      case "$words[3]" in
        init)
          _arguments \
            '--session[Set session id]:session-id:_guard "[^ ]#" "session"' \
            '--last-session[Reuse last session]' \
            '--codex-binary[Set Codex CLI path]:cmd:_files'
          ;;
        ask)
          _arguments \
            '--session[Override session]:session-id:_guard "[^ ]#" "session"' \
            '--codex-binary[Set Codex CLI path]:cmd:_files' \
            '--timeout[Set timeout seconds]:seconds:_guard "[0-9]#" seconds' \
            '--show-token-cost[Print token usage]'
          ;;
        tui)
          _arguments
          ;;
        *)
          _values 'codex command' init ask tui
          ;;
      esac
      ;;
    completion)
      if (( CURRENT == 3 )); then
        _values 'shell' bash zsh fish
      else
        _arguments \
          '--dir[Target directory]:dir:_path_files -/' \
          '--print[Print script only]'
      fi
      ;;
    *)
      _describe 'command' commands
      ;;
  esac
}

_jeff "$@"
`

const fishScript = `# fish completion for jeff
function __jeff_using_command
    set -l cmd (commandline -opc)
    if test (count $cmd) -ge 2
        if test $cmd[2] = $argv[1]
            return 0
        end
    end
    return 1
end

function __jeff_using_codex_subcommand
    set -l cmd (commandline -opc)
    if test (count $cmd) -ge 3
        if test $cmd[2] = codex
            if test $cmd[3] = $argv[1]
                return 0
            end
        end
    end
    return 1
end

complete -c jeff -n '__fish_use_subcommand' -a 'codex' -d 'Codex commands'
complete -c jeff -n '__fish_use_subcommand' -a 'completion' -d 'Generate completions'
complete -c jeff -n '__fish_use_subcommand' -a 'help' -d 'Show help'

complete -c jeff -n '__jeff_using_command codex' -a 'init' -d 'Configure Codex session'
complete -c jeff -n '__jeff_using_command codex' -a 'ask' -d 'Ask Codex'
complete -c jeff -n '__jeff_using_command codex' -a 'tui' -d 'Interactive session'

complete -c jeff -n '__jeff_using_codex_subcommand init' -l session -d 'Set session id' -r
complete -c jeff -n '__jeff_using_codex_subcommand init' -l last-session -d 'Reuse last session'
complete -c jeff -n '__jeff_using_codex_subcommand init' -l codex-binary -d 'Codex CLI path' -r

complete -c jeff -n '__jeff_using_codex_subcommand ask' -l session -d 'Override session id' -r
complete -c jeff -n '__jeff_using_codex_subcommand ask' -l codex-binary -d 'Codex CLI path' -r
complete -c jeff -n '__jeff_using_codex_subcommand ask' -l timeout -d 'Response timeout seconds' -r
complete -c jeff -n '__jeff_using_codex_subcommand ask' -l show-token-cost -d 'Print token usage'

complete -c jeff -n '__jeff_using_command completion' -a 'bash' -d 'bash completion'
complete -c jeff -n '__jeff_using_command completion' -a 'zsh' -d 'zsh completion'
complete -c jeff -n '__jeff_using_command completion' -a 'fish' -d 'fish completion'
complete -c jeff -n '__jeff_using_command completion' -l dir -d 'Target directory' -r
complete -c jeff -n '__jeff_using_command completion' -l print -d 'Print script only'
`
