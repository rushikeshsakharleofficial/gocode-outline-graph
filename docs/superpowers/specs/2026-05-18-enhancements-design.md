# Enhancement Design: Correctness Fixes, MCP Tool Improvements, Language Cleanup

**Date:** 2026-05-18  
**Approach:** Phased — bugs first, then MCP usability, then language cleanup  
**Compatibility:** Additive only — no existing tool signatures broken

---

## Scope

Three independent, shippable phases:

| Phase | Focus | Risk |
|-------|-------|------|
| 1 | Bug fixes (correctness) | Low — no protocol surface changes |
| 2 | MCP tool enhancements | Low — additive params + new tools only |
| 3 | Language extension cleanup | Low — removes broken no-op indexing |

---

## Phase 1 — Bug Fixes

### 1a. Fix `languages.go` — truthful `IsSupported`

**Problem:** `extensionMap` maps 20+ extensions (`.json`, `.yaml`, `.html`, `.css`, `.toml`, `.md`, `.sql`, `.lua`, `.cs`, `.kt`, `.swift`, `.php`, `.r`, `.dart`, `.zig`, `.vue`, `.svelte`, `.scala`) to language names, but `parser.go`'s `Parse()` switch only has cases for 11 languages. All others hit `return nil`. `IsSupported` returns `true`, the indexer reads and checksums those files, writes an `indexed_files` row, and inserts zero symbols — wasted I/O and misleading `status` output.

**Fix:** Remove all extensions whose target language has no `case` in `Parse()` from `extensionMap`. Only retain the 11 implemented grammars:
- Python (`.py`)
- JavaScript (`.js`, `.mjs`, `.cjs`)
- TypeScript (`.ts`, `.tsx`)
- Go (`.go`)
- Rust (`.rs`)
- Java (`.java`)
- C (`.c`, `.h`)
- C++ (`.cpp`, `.cc`, `.cxx`, `.hpp`, `.hh`, `.hxx`)
- Ruby (`.rb`)
- Bash (`.sh`, `.bash`)

**Files:** `internal/parser/languages.go`

---

### 1b. Fix `cmdStatus` — efficiency

**Problem:** `cmdStatus` calls `s.KeywordSearch("", 10000)` which is a `LIKE '%%'` full-table scan returning up to 10,000 rows into memory, then uses an O(n²) bubble sort to find the top 10 files.

**Fix:** Replace the entire `allSymbols` accumulation block with two existing DB methods:
```go
fileCount, symbolCount, err := database.GetFileStats()
topFiles, err := database.GetTopFiles(10)
```
Both are already implemented and use SQL aggregation. Eliminates the in-memory scan and sort entirely.

**Files:** `cmd/code-outline-graph/main.go` (`cmdStatus`)

---

### 1c. Implement `--force` flag

**Problem:** `--force` is parsed by `parseFlags` but never used — `_ = f.force` comment reads "future: pass force flag to indexer". The flag has no effect.

**Fix:** Add `Force bool` field to `Indexer`. When `Force` is true, `indexFileWithParser` skips the `IsFileCurrent` fast-path and the checksum double-check, always re-parsing. After `idx := indexer.New(database, f.workers)`, set `idx.Force = f.force` in `cmdBuild` and `cmdUpdate`.

**Files:** `internal/indexer/indexer.go`, `cmd/code-outline-graph/main.go`

---

### 1d. Fix `find_callees` — resolve callee names to symbols

**Problem:** `GetCallees(callerID)` returns `[]string` (bare callee names). The MCP handler prints only names with no file/line. An LLM agent cannot navigate from a bare function name without another lookup.

**Fix:** Add `GetCalleeSymbols(callerID int64) ([]Symbol, error)` to `Database` using a JOIN:
```sql
SELECT s.<cols>
FROM call_graph cg
JOIN symbols s ON s.name = cg.callee_name
WHERE cg.caller_id = ?
ORDER BY s.file_path, s.start_line
```
For callee names with no matching symbol (external/stdlib calls), append a "unresolved: <name>" line. Update `handleFindCallees` to use the new method and display `file:line kind name` format, matching `find_callers` output.

**Files:** `internal/db/db.go`, `internal/server/tools.go`

---

### 1e. Fix watcher — watch newly-created directories

**Problem:** `Start()` adds all existing subdirectories to `fsnotify` at startup. If a new directory is created after startup (e.g. `mkdir src/newpkg`), it is never added to the watcher — files inside it are silently ignored.

**Fix:** In `handleEvent`, when `op&fsnotify.Create != 0`, check if the path is a directory (`os.Stat` → `IsDir()`). If so, call `w.watcher.Add(path)` to register it. This ensures new packages/modules are watched immediately.

**Files:** `internal/watcher/watcher.go`

---

## Phase 2 — MCP Tool Enhancements

All changes are additive only. No existing param is removed or renamed.

### 2a. `find_by_keyword` — add `kind`, `language`, `file_pattern` optional filters

**New optional params:**
- `kind` (string) — restrict to a symbol kind: `function`, `class`, `method`, `struct`, `interface`, `type`, `constant`, `module`, `enum`, `decorator`
- `language` (string) — restrict to a language: `go`, `python`, `typescript`, etc.
- `file_pattern` (string) — SQLite GLOB pattern matched against `file_path`, e.g. `*/handlers/*.go` (SQLite GLOB uses `*`/`?`, is case-sensitive on Linux)

**DB layer:** Add `FilteredSearch(query, kind, language, filePattern string, limit int) ([]Symbol, error)` that appends `AND kind = ?` / `AND language = ?` / `AND file_path GLOB ?` clauses to the existing FTS + keyword search paths.

**Files:** `internal/db/db.go`, `internal/server/tools.go`

---

### 2b. `get_symbol` — return all matches when file unspecified

**Problem:** `GetSymbolByName` uses `LIMIT 1` and silently returns an arbitrary first match when multiple files define the same name (common in monorepos and multi-package Go projects).

**Fix:** Add `GetSymbolsByName(name, filePath string) ([]Symbol, error)` returning all matches. `handleGetSymbol` behavior:
- `file_path` specified → existing single-result behavior (backward compat)
- `file_path` unspecified, 1 match → returns a single JSON object (backward compat)
- `file_path` unspecified, N > 1 matches → returns a JSON array with all candidates

**Files:** `internal/db/db.go`, `internal/server/tools.go`

---

### 2c. `read_symbol_body` — add `context_lines` optional param

**New optional param:** `context_lines` (integer, default 0). Expands the read range by N lines before `start_line` and N lines after `end_line`. Useful when an LLM needs surrounding context (e.g. the struct field list above a method receiver).

**Files:** `internal/server/tools.go`

---

### 2d. New tool: `list_files`

```
list_files(project_path, file_pattern?)
```

Returns sorted list of all indexed file paths. Optional `file_pattern` (glob) filters the list. Backed by `ListIndexedFiles()` which already exists. Replaces the awkward `get_outline_summary` for "what files are indexed?" queries.

**Files:** `internal/server/tools.go` (new handler + schema entry)

---

### 2e. New tool: `find_by_kind`

```
find_by_kind(kind, project_path, language?)
```

Returns all symbols of a given kind across the project. Hard limit: 200 results. Backed by new `GetSymbolsByKind(kind, language string, limit int) ([]Symbol, error)` DB method. Useful for "list all exported functions" or "find all classes" queries.

**Files:** `internal/db/db.go`, `internal/server/tools.go`

---

## Phase 3 — Language Extension Cleanup

### 3a. Trim `extensionMap` to only parsed languages

Remove from `extensionMap`:
`.json`, `.yaml`, `.yml`, `.html`, `.htm`, `.css`, `.lua`, `.cs`, `.toml`, `.md`, `.markdown`, `.sql`, `.scala`, `.kt`, `.kts`, `.swift`, `.php`, `.r`, `.R`, `.dart`, `.zig`, `.vue`, `.svelte`

Also remove `"json"`, `"yaml"`, `"html"`, `"css"`, `"lua"`, `"c_sharp"`, `"toml"`, `"markdown"`, `"sql"`, `"scala"`, `"kotlin"`, `"swift"`, `"php"`, `"r"`, `"dart"`, `"zig"`, `"vue"`, `"svelte"` from `basenameMap` if present.

**Effect:** `IsSupported` becomes truthful. Zero-symbol `indexed_files` rows for these extensions will be stale — users should run `code-outline-graph update .` (which calls `RemoveStale`) once after upgrading.

**Files:** `internal/parser/languages.go`

### 3b. Release note

Document in CHANGELOG or commit message: existing indexes may have ghost entries for previously-indexed files with these extensions. Run `update .` or `prune .` once after upgrading.

---

## Data Flow Summary

```
Phase 1 touches:
  parser/languages.go      → IsSupported truthful
  indexer/indexer.go       → Force field
  db/db.go                 → GetCalleeSymbols
  server/tools.go          → handleFindCallees updated
  watcher/watcher.go       → watch new dirs
  cmd/.../main.go          → cmdStatus fixed, --force wired

Phase 2 touches:
  db/db.go                 → FilteredSearch, GetSymbolsByName, GetSymbolsByKind
  server/tools.go          → updated handlers + 2 new tools

Phase 3 touches:
  parser/languages.go      → trim extensionMap
```

---

## Non-Goals

- HTTP/SSE transport (out of scope)
- Multi-project server (out of scope)
- Adding new language grammars (out of scope — use existing 11 only)
- Changing any existing required parameter
