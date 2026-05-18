# Go Python Parity Design

**Date:** 2026-05-18  
**Goal:** Bring Go version of code-outline-graph to full feature parity with the Python production version.

---

## Background

The Python version (`../code-outline-graph/`) is stable and in production. The Go version is a port but lags in three areas: language support (12 vs 40+ languages), missing CLI commands, and a missing MCP tool. This spec covers all three gaps.

Intentional divergence from Python (kept): vector search (fastembed/sqlite-vec) was dropped from Go. The Go version has two extra MCP tools (`list_files`, `find_by_kind`) not in Python — these are kept.

---

## Gap Summary

| Area | Python | Go (current) | Action |
|---|---|---|---|
| Languages | 40+ | 12 | Add 30+ via tree-sitter + fallback |
| CLI commands | 13 | 10 | Add doctor, export, callers, callees |
| MCP tools | 13 | 14 | Add resolve_edit_target |
| Symbol kinds | includes import, section, table, view | missing these 4 | Add to relevant walkers |

---

## 1. Language Expansion

### Approach

Use smacker/go-tree-sitter grammars (already vendored) for all languages that have them. For languages without a Go tree-sitter binding, use a generic line-based fallback that produces `section` kind symbols — identical to how Python handles config/markup files internally.

### Parser File Reorganization

Split `internal/parser/parser.go` (797 lines) into focused files:

```
internal/parser/
  parser.go          — SymbolParser type, dispatch table, New(), Parse(), ParseWithCalls(), shared helpers
  walk_systems.go    — walkCSharp, walkPHP, walkSwift, walkKotlin, walkScala
  walk_config.go     — walkYAML, walkTOML, walkHCL, walkProtobuf, walkJSON (stdlib encoding/json)
  walk_web.go        — walkHTML, walkCSS, walkSvelte
  walk_scripting.go  — walkLua, walkElixir, walkGroovy, walkOCaml
  walk_markup.go     — walkMarkdown
  walk_fallback.go   — walkLineBased (dart, zig, erlang, clojure, haskell, nix, fish, perl, r, powershell, batch, graphql, vue, xml, etc.)
  languages.go       — extensionMap + basenameMap + DetectLanguage + IsSupported
  languages_test.go  — existing tests + new language assertions
```

### New Languages — Tree-sitter Walkers

These have grammars in smacker/go-tree-sitter and get full AST-quality symbol extraction:

| Language | Extensions | Walk function | Symbol kinds extracted |
|---|---|---|---|
| csharp | .cs | walkCSharp | class, interface, method, function, constant, enum, struct |
| php | .php | walkPHP | class, function, method, interface, constant |
| swift | .swift | walkSwift | class, struct, function, method, constant, enum, interface (protocol) |
| kotlin | .kt .kts | walkKotlin | class, function, method, constant, interface, enum |
| scala | .scala | walkScala | class, function, method, object, constant |
| yaml | .yaml .yml | walkYAML | section (top-level keys) |
| toml | .toml | walkTOML | section (table headers + keys) |
| hcl | .tf .hcl | walkHCL | section (resource/module blocks) |
| protobuf | .proto | walkProtobuf | class (message), function (rpc), constant (field), enum |
| html | .html .htm | walkHTML | section (h1-h6 headings), function (script blocks), constant (meta/link elements) |
| css | .css | walkCSS | section (rule selectors, @media blocks, @keyframes) |
| svelte | .svelte | walkSvelte | section (script, style, and template root blocks) |
| lua | .lua | walkLua | function, method, constant |
| elixir | .ex .exs | walkElixir | module, function, method |
| groovy | .groovy .gradle | walkGroovy | class, function, method |
| ocaml | .ml .mli | walkOCaml | function, module, type, constant |
| markdown | .md .mdx | walkMarkdown | section (ATX/setext headings, depth as metadata) |
| json | .json | walkJSON | section (top-level object keys via encoding/json; nested objects become child sections) |
| sql | .sql | walkSQL | table (CREATE TABLE), view (CREATE VIEW), function (CREATE FUNCTION/PROCEDURE) |
| dockerfile | Dockerfile | walkDockerfile | section (FROM, RUN, ENTRYPOINT stages) |

scss/sass/less: `extensionMap` maps `.scss`, `.sass`, `.less` → language name `"css"` so they use the css grammar walker.  
vue: `extensionMap` maps `.vue` → language name `"svelte"` so they use the svelte grammar walker (same JS+template structure).  
elm: available in smacker/go-tree-sitter but NOT in Python's language list — not added (exact parity goal).

### New Languages — Line-based Fallback

These have no Go tree-sitter binding; produce `section` kind symbols from structural markers:

dart (.dart), zig (.zig), clojure (.clj .cljs .cljc), erlang (.erl .hrl), haskell (.hs .lhs), nix (.nix), fish (.fish), perl (.pl .pm .t), r (.r .R), powershell (.ps1 .psm1), batch (.bat .cmd), graphql (.graphql .gql), xml (.xml), make (Makefile .mk), sqlite (inline schema)

**Line-based fallback rule:** scan source lines for language-specific structural markers (function/class/def/module/section keywords or markdown-style headings). Each matched line becomes a `section` symbol spanning from that line to the next marker or end-of-file.

### New Symbol Kinds

Add to existing walkers and fallback:

- `import` — add to Python walker (import_statement, import_from_statement), JS/TS walker (import_declaration)
- `section` — used by all config/markup/fallback walkers
- `table` — SQL walker (CREATE TABLE)
- `view` — SQL walker (CREATE VIEW)

### languages.go Changes

Add ~50 new entries to `extensionMap` covering all new languages. Add new entries to `basenameMap` (Makefile, Dockerfile variations). `IsSupported()` returns true for all mapped languages.

---

## 2. CLI Additions

All four new subcommands added to `cmd/code-outline-graph/main.go`.

### `doctor <path>`

Health check that verifies the installation is working correctly.

```
code-outline-graph-go doctor <path>
```

Checks (pass/fail per line):
1. DB file exists at `<path>/.code-outline-graph/index.db`
2. SQLite `PRAGMA integrity_check` returns "ok"
3. `indexed_files` and `symbols` tables exist with row counts
4. SymbolParser initializes for all 12+ languages (instantiate New(), no panic)
5. MCP config files exist (`.mcp.json`, `.claude/settings.json`)

Exits 0 if all pass, 1 if any fail.

### `export <path> [--format json|csv] [--output file]`

Dump all indexed symbols to JSON or CSV.

```
code-outline-graph-go export <path> [--format json|csv] [--output output.json]
```

- Default format: json
- Default output: stdout
- Fields: id, name, kind, file_path, start_line, end_line, signature, docstring, parent, language

### `callers <path> <name>`

Print all symbols that call `<name>`.

```
code-outline-graph-go callers <path> <function-name>
```

Calls `db.GetCallersByName(name)`. Prints one result per line: `file_path:start_line  caller_name (kind)`.

### `callees <path> <name>`

Print all functions called by `<name>`.

```
code-outline-graph-go callees <path> <symbol-name>
```

Resolves symbol by name, then calls `db.GetCalleeSymbols(id)`. Same output format as callers.

---

## 3. MCP Tool: `resolve_edit_target`

Wire the existing `searcher.ResolveEditTarget()` RRF logic (already in `internal/search/search.go`) as an MCP tool.

**Tool definition** (add to `allTools()` in `internal/server/tools.go`):

```
name: resolve_edit_target
description: Find the best edit targets for a natural language query. Returns top candidates ranked by hybrid FTS+keyword search with Reciprocal Rank Fusion.
args:
  query       string (required) — natural language description of what to edit
  project_path string (optional) — defaults to current directory
  limit       int (default 5) — number of candidates to return
returns: ranked list of symbols with name, kind, file_path, start_line, end_line, signature
```

**Handler** `handleResolveEditTarget()` in `tools.go`:
1. Open DB for project_path
2. Create `Searcher` wrapping DB
3. Call `searcher.ResolveEditTarget(query, limit)`
4. Format results as text (same pattern as other tool handlers)

No new packages or schema changes required.

---

## 4. Data Flow

No schema changes required. All new languages map to existing symbol kinds (or the new `import`/`section`/`table`/`view` kinds added to the DB symbol kind set). The `call_graph` table gains call edges for languages where `ParseWithCalls()` support is added (c#, kotlin, swift, scala, lua, elixir).

Call extraction for new languages: add to `ParseWithCalls()` dispatch — walk `call_expression` / `method_invocation` nodes for each language where the grammar supports it.

---

## 5. Error Handling

- New walk functions follow existing convention: return `nil` (not error) on parse failure; caller logs and skips
- Line-based fallback: never errors; always returns at least one section symbol for non-empty files
- `doctor` command: errors are non-fatal per check; summarize all and exit with count
- `export` command: fail fast on DB open error; stream results (don't hold all in memory)

---

## 6. Testing

- `internal/parser/languages_test.go` — add assertions for all new extensions
- Each new walk function: at minimum one fixture test with a small source snippet
- `internal/parser/parser_test.go` (new) — integration test: parse a file of each new language, assert ≥1 symbol returned
- CLI integration: `go build` smoke test after changes

---

## 7. Dependency Changes

No new Go module dependencies. All new tree-sitter grammars are already in `github.com/smacker/go-tree-sitter@v0.0.0-20240827094217-dd81d9e9be82` which is already in go.mod. Only new import paths need to be added to the Go source files:

```go
import (
    sitter_csharp "github.com/smacker/go-tree-sitter/csharp"
    sitter_php    "github.com/smacker/go-tree-sitter/php"
    sitter_swift  "github.com/smacker/go-tree-sitter/swift"
    // ... etc
)
```

---

## 8. Implementation Order

1. `languages.go` — add all new extension/basename mappings (foundation for everything else)
2. `walk_config.go` — yaml, toml, hcl, protobuf, json (high value, common files)
3. `walk_systems.go` — csharp, php, swift, kotlin, scala (most-requested languages)
4. `walk_web.go` — html, css, svelte
5. `walk_scripting.go` — lua, elixir, groovy, ocaml
6. `walk_markup.go` — markdown
7. `walk_fallback.go` — all remaining languages
8. Update `parser.go` dispatch table + `New()` to register all new parsers
9. MCP tool: `resolve_edit_target`
10. CLI: doctor, export, callers, callees
11. Tests
