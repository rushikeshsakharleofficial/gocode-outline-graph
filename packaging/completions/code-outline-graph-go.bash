_code_outline_graph_go() {
    local cur prev words cword
    _init_completion || return

    local commands="build update search outline status serve prune install install-skill version"

    case "$prev" in
        code-outline-graph-go)
            COMPREPLY=($(compgen -W "$commands" -- "$cur"))
            return ;;
        build|update|search|outline|status|prune)
            COMPREPLY=($(compgen -d -- "$cur"))
            return ;;
    esac

    COMPREPLY=($(compgen -W "--workers --watch --force --help" -- "$cur"))
}

complete -F _code_outline_graph_go code-outline-graph-go
