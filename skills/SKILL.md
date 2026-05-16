# code-outline-graph-go Skill

**MANDATORY: Use this before any file read, grep, or search operation.**

## Hard Rule

```
NEVER use Read/Grep/Glob on source files without first checking the outline index.
```

If you're about to read a file → STOP → use `list_outline` or `find_by_keyword` first.

---

## The Confirm-Before-Read Protocol

```
Step 1 — Find (no body, just metadata):
  find_by_keyword({"query": "authentication middleware", "project_path": "."})
  → [{name: "authenticate", file: "auth.go", start: 45, end: 89,
      signature: "func authenticate(token string) (*User, error)"}]

Step 2 — Confirm (pick correct candidate from signatures)

Step 3 — Read ONLY that body:
  read_symbol_body({"name": "authenticate", "file_path": "auth.go", "project_path": "."})
  → 44 lines instead of 300-line file
```

Token savings: 10x–50x per edit.

---

## Tool Reference

| What you need | Tool to use |
|---|---|
| Find function/class to edit | `find_by_keyword({"query": "...", "project_path": "."})` |
| Read one function body | `read_symbol_body({"name": "...", "file_path": "...", "project_path": "."})` |
| All symbols in file | `list_outline({"file_path": "...", "project_path": "."})` |
| Project overview | `get_outline_summary({"project_path": "."})` |
| Imports + top of file | `get_file_header({"file_path": "...", "project_path": "."})` |
| Exact symbol metadata | `get_symbol({"name": "...", "project_path": "."})` |
| Read specific lines | `get_line_range({"file_path": "...", "start_line": N, "end_line": M})` |
| Who calls a function? | `find_callers({"name": "...", "project_path": "."})` |
| What does a function call? | `find_callees({"symbol_name": "...", "project_path": "."})` |
| Index a project | `index_project({"project_path": "."})` |
| Reindex changed files | `update_project({"project_path": "."})` |
| Remove stale entries | `prune_project({"project_path": "."})` |

---

## Patterns by Task

### Editing a function
```
find_by_keyword({"query": "user login handler", "project_path": "."})
→ pick from candidates using signatures
read_symbol_body({"name": "Login", "file_path": "handlers/auth.go", "project_path": "."})
→ edit with exact line range from start_line/end_line
```

### Understanding a file
```
list_outline({"file_path": "src/server.go", "project_path": "."})
→ all functions, types, methods at a glance
read_symbol_body only for what you need
```

### Tracing a call chain
```
find_callers({"name": "InsertSymbolsForFile", "project_path": "."})
→ every function that calls this one

find_callees({"symbol_name": "IndexAll", "project_path": "."})
→ everything IndexAll calls
```

### Getting imports / top-of-file context
```
get_file_header({"file_path": "cmd/main.go", "project_path": "."})
→ package + imports + top-level constants only
```

### Project overview before diving in
```
get_outline_summary({"project_path": "."})
→ file count, symbol count, top files by density
```

---

## After Every Code Change

**MANDATORY:** After editing or creating any source file, run:

```
update_project({"project_path": "."})
```

This keeps the index current so future symbol lookups reflect your changes.

---

## When to Fall Back to Read/Grep

Only if:
- `find_by_keyword` returns empty results
- `list_outline` returns no symbols (config/JSON/YAML file)
- Symbol body needed is a generated or minified file

---

## CLI Commands (terminal)

```bash
code-outline-graph-go build .          # full index
code-outline-graph-go update .         # reindex changed files
code-outline-graph-go search . <query> # search from terminal
code-outline-graph-go outline . <file> # show file symbols
code-outline-graph-go status .         # index stats
code-outline-graph-go serve            # start MCP server (stdio)
code-outline-graph-go prune .          # remove stale entries
code-outline-graph-go install          # write MCP configs for editors
code-outline-graph-go install-skill    # install this skill to ~/.claude/skills/
```
