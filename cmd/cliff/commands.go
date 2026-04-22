package main

import (
	"fmt"
	"os"
)

// version is overridden at build time via:
//   go build -ldflags "-X main.version=v0.1.0"
var version = "dev"

func cmdVersion() int {
	fmt.Println("cliff", version)
	return 0
}

func cmdHelp() int {
	fmt.Print(helpText)
	return 0
}

const helpText = `cliff — a terminal-native directory for CLIs and TUIs.

Usage:
  cliff                        launch the browser (TUI)
  cliff install <pkg>          install an app via its package manager
  cliff uninstall <pkg>        uninstall a previously installed app
  cliff upgrade <pkg>          upgrade an app to its latest version
  cliff submit [name|repo]     nominate an app for the cliff registry
  cliff help                   show this message
  cliff version                print the installed version
  cliff completions <shell>    emit shell completion script
                               (run 'cliff completions' for install tips)

See https://cliff.sh for more.
`

const completionsHelpText = `cliff completions — emit a shell completion script.

Usage:
  cliff completions <shell>    print the completion script to stdout
                               (shell: bash, zsh, or fish)

Install (bash):
  # Ephemeral — current shell only:
  eval "$(cliff completions bash)"
  # Persistent — add the line above to ~/.bashrc.

Install (zsh):
  # Drop it on $fpath (oh-my-zsh users):
  cliff completions zsh > ~/.oh-my-zsh/custom/completions/_cliff

  # Or plain zsh — ensure the dir is on fpath before compinit in ~/.zshrc:
  mkdir -p ~/.zsh/completions
  cliff completions zsh > ~/.zsh/completions/_cliff
  # then in ~/.zshrc:
  #   fpath=(~/.zsh/completions $fpath)
  #   autoload -Uz compinit && compinit

  # Reload: exec zsh  (or start a new shell).

Install (fish):
  cliff completions fish > ~/.config/fish/completions/cliff.fish

After install, 'cliff <TAB>' offers the known verbs.
`

func cmdCompletions(args []string) int {
	if len(args) == 0 {
		fmt.Print(completionsHelpText)
		return 0
	}
	switch args[0] {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	case "help", "--help", "-h":
		fmt.Print(completionsHelpText)
	default:
		fmt.Fprintf(os.Stderr, "cliff: unknown shell %q (supported: bash, zsh, fish)\n", args[0])
		fmt.Fprintln(os.Stderr, "run 'cliff completions' for install instructions")
		return 2
	}
	return 0
}

const bashCompletion = `# cliff bash completion
# Install: add this to ~/.bashrc, or source from a completions dir
#   eval "$(cliff completions bash)"

_cliff() {
    local cur prev verbs
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    verbs="install uninstall upgrade submit completions help version"

    if [ "$COMP_CWORD" -eq 1 ]; then
        COMPREPLY=( $(compgen -W "$verbs" -- "$cur") )
        return 0
    fi

    case "$prev" in
        completions)
            COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
            return 0
            ;;
    esac
}
complete -F _cliff cliff
`

const zshCompletion = `#compdef cliff
# cliff zsh completion
# Install: place this file as _cliff somewhere on $fpath, e.g.
#   cliff completions zsh > ~/.zsh/completions/_cliff
# Then ensure ~/.zsh/completions is on fpath in ~/.zshrc:
#   fpath=(~/.zsh/completions $fpath)
#   autoload -Uz compinit && compinit

_cliff() {
    local -a verbs
    verbs=(
        'install:install an app via its package manager'
        'uninstall:uninstall a previously installed app'
        'upgrade:upgrade an app to its latest version'
        'submit:nominate an app for the cliff registry'
        'completions:emit shell completion script'
        'help:show help'
        'version:print the installed version'
    )

    if (( CURRENT == 2 )); then
        _describe 'verb' verbs
        return
    fi

    case "${words[2]}" in
        completions)
            if (( CURRENT == 3 )); then
                _values 'shell' 'bash' 'zsh' 'fish'
            fi
            ;;
    esac
}

_cliff "$@"
`

const fishCompletion = `# cliff fish completion
# Install: save this to ~/.config/fish/completions/cliff.fish
#   cliff completions fish > ~/.config/fish/completions/cliff.fish

complete -c cliff -f

complete -c cliff -n '__fish_use_subcommand' -a 'install'     -d 'install an app via its package manager'
complete -c cliff -n '__fish_use_subcommand' -a 'uninstall'   -d 'uninstall a previously installed app'
complete -c cliff -n '__fish_use_subcommand' -a 'upgrade'     -d 'upgrade an app to its latest version'
complete -c cliff -n '__fish_use_subcommand' -a 'submit'      -d 'nominate an app for the cliff registry'
complete -c cliff -n '__fish_use_subcommand' -a 'completions' -d 'emit shell completion script'
complete -c cliff -n '__fish_use_subcommand' -a 'help'        -d 'show help'
complete -c cliff -n '__fish_use_subcommand' -a 'version'     -d 'print the installed version'

complete -c cliff -n '__fish_seen_subcommand_from completions' -a 'bash zsh fish'
`
