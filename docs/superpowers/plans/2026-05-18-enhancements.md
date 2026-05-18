# Enhancements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix correctness bugs, add MCP tool filters and new tools, and trim the extension map to only languages with real parser implementations.

**Architecture:** Three phases executed in order. Phase 1 (bug fixes) — pure correctness, no protocol surface changes. Phase 2 (MCP enhancements) — additive-only new params and new tools. Phase 3 (language cleanup) — removes broken no-op indexing. All changes stay backward-compatible for existing MCP clients.

**Tech Stack:** Go, SQLite (go-sqlite3 with `-tags fts5`), tree-sitter (go-tree-sitter), fsnotify. Build: `go build -buildvcs=false -tags fts5 -o code-outline-graph ./cmd/code-outline-graph/`. Test: `go test -tags fts5 ./...`.

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Modify | `internal/parser/languages.go` | Trim extensionMap to 11 implemented grammars (Tasks 1 + 11) |
| Create | `internal/parser/languages_test.go` | IsSupported correctness tests |
| Modify | `internal/db/db.go` | Add FileCount type, GetTopFilesWithCounts, GetCalleeSymbols, FilteredSearch, GetSymbolsByName, GetSymbolsByKind |
| Create | `internal/db/db_test.go` | DB method unit tests (shared helper openTestDB) |
| Modify | `internal/indexer/indexer.go` | Add Force bool field, wire into fast-path skips |
| Create | `internal/indexer/indexer_test.go` | Force flag behavioral test |
| Modify | `internal/watcher/watcher.go` | Add new-directory detection in handleEvent |
| Modify | `internal/server/tools.go` | Update find_by_keyword, get_symbol, read_symbol_body, find_callees handlers; add list_files and find_by_kind handlers and schemas |
| Modify | `cmd/code-outline-graph/main.go` | Fix cmdStatus to use GetFileStats + GetTopFilesWithCounts; wire idx.Force |

---

## Phase 1 — Bug Fixes

---

### Task 1: Fix IsSupported — remove unimplemented language extensions

`extensionMap` maps 20+ extensions to language names that have no `case` in `parser.go`'s `Parse()` switch. `IsSupported` returns true, the indexer reads and checksums files, stores `indexed_files` rows, and inserts zero symbols — wasted I/O and misleading status output.

**Note:** This task and Task 11 both touch `languages.go`. Do them together here; Task 11 is a no-op if you complete this task fully.

**Files:**
- Modify: `internal/parser/languages.go`
- Create: `internal/parser/languages_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/parser/languages_test.go`:

```go
package parser_test

import (
	"testing"

	"gocode-outline-graph/internal/parser"
)

func TestIsSupported_acceptsImplementedLanguages(t *testing.T) {
	cases := []string{
		"main.py", "index.js", "app.mjs", "mod.cjs",
		"types.ts", "App.tsx", "main.go", "lib.rs",
		"Hello.java", "util.c", "util.h",
		"src.cpp", "src.cc", "src.cxx", "src.hpp", "src.hh", "src.hxx",
		"script.rb", "run.sh", "run.bash",
	}
	for _, f := range cases {
		if !parser.IsSupported(f) {
			t.Errorf("expected %q to be supported", f)
		}
	}
}

func TestIsSupported_rejectsBrokenExtensions(t *testing.T) {
	cases := []string{
		"data.json", "config.yaml", "config.yml",
		"page.html", "page.htm", "style.css",
		"notes.md", "notes.markdown",
		"query.sql", "App.vue", "App.svelte",
		"lib.lua", "Prog.cs", "Main.kt", "Main.kts",
		"App.swift", "lib.php", "analysis.r", "analysis.R",
		"lib.dart", "main.zig", "lib.toml", "Main.scala",
	}
	for _, f := range cases {
		if parser.IsSupported(f) {
			t.Errorf("expected %q to NOT be supported", f)
		}
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test -tags fts5 ./internal/parser/ -run TestIsSupported -v
```

Expected: `TestIsSupported_rejectsBrokenExtensions` FAIL — currently returns true for `.json`, `.yaml`, etc.

- [ ] **Step 3: Replace extensionMap with only the 11 implemented grammars**

Replace the entire `extensionMap` and `basenameMap` in `internal/parser/languages.go`:

```go
// extensionMap maps lowercase file extensions (with leading dot) to tree-sitter language names.
// Only contains extensions whose language has a case in Parse().
var extensionMap = map[string]string{
	".py":   "python",
	".js":   "javascript",
	".mjs":  "javascript",
	".cjs":  "javascript",
	".ts":   "typescript",
	".tsx":  "tsx",
	".go":   "go",
	".rs":   "rust",
	".java": "java",
	".c":    "c",
	".h":    "c",
	".cpp":  "cpp",
	".cc":   "cpp",
	".cxx":  "cpp",
	".hpp":  "cpp",
	".hh":   "cpp",
	".hxx":  "cpp",
	".rb":   "ruby",
	".sh":   "bash",
	".bash": "bash",
}

// basenameMap maps exact filenames (no path, case-sensitive) to language names.
var basenameMap = map[string]string{
	"Dockerfile": "dockerfile",
	"dockerfile": "dockerfile",
}
```

Note: `dockerfile` has no `case` in `Parse()` either, but it maps to an unimplemented grammar — it will be handled by `IsSupported` returning true but `Parse()` returning nil (zero symbols). This is acceptable since Dockerfile is rarely the target of symbol search. If needed, remove `dockerfile` entries too.

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test -tags fts5 ./internal/parser/ -run TestIsSupported -v
```

Expected: PASS for both tests.

- [ ] **Step 5: Build to verify compilation**

```bash
go build -buildvcs=false -tags fts5 -o code-outline-graph ./cmd/code-outline-graph/
```

Expected: exits 0, binary created.

- [ ] **Step 6: Commit**

```bash
git add internal/parser/languages.go internal/parser/languages_test.go
git commit -m "fix(parser): restrict IsSupported to 11 implemented grammars

Removes 20+ extensions (.json, .yaml, .html, .css, .md, .sql, .vue,
.svelte, .lua, .cs, .kt, .swift, .php, .r, .dart, .zig, .toml,
.scala) that had no parser implementation. IsSupported now returns
true only for languages with a case in Parse().

Existing indexes may have ghost indexed_files rows for these extensions.
Run 'update .' or 'prune .' once after upgrading.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 2: Fix cmdStatus — replace full-table scan + bubble sort with DB aggregation

`cmdStatus` calls `KeywordSearch("", 10000)` (a `LIKE '%%'` full-table scan) then O(n²) bubble-sorts the results. `GetFileStats()` and a new `GetTopFilesWithCounts()` method do the same in SQL.

**Files:**
- Modify: `internal/db/db.go` (add `FileCount` type + `GetTopFilesWithCounts`)
- Create: `internal/db/db_test.go` (add `openTestDB` helper + test for new method)
- Modify: `cmd/code-outline-graph/main.go` (rewrite `cmdStatus`)

- [ ] **Step 1: Write failing test for GetTopFilesWithCounts**

Create `internal/db/db_test.go`:

```go
package db_test

import (
	"path/filepath"
	"testing"
	"time"

	"gocode-outline-graph/internal/db"
)

// openTestDB creates a temporary SQLite DB for the duration of t.
func openTestDB(t *testing.T) *db.Database {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

// insertSymbols inserts symbols into the DB for filePath.
func insertSymbols(t *testing.T, d *db.Database, filePath string, syms []db.Symbol) {
	t.Helper()
	if err := d.InsertSymbolsForFile(filePath, syms, "checksum-"+filePath, time.Now().UnixNano(), 100); err != nil {
		t.Fatalf("insert symbols for %q: %v", filePath, err)
	}
}

func TestGetTopFilesWithCounts(t *testing.T) {
	d := openTestDB(t)

	insertSymbols(t, d, "a.go", []db.Symbol{
		{Name: "Foo", Kind: "function", Language: "go"},
		{Name: "Bar", Kind: "function", Language: "go"},
		{Name: "Baz", Kind: "function", Language: "go"},
	})
	insertSymbols(t, d, "b.go", []db.Symbol{
		{Name: "Qux", Kind: "function", Language: "go"},
	})

	results, err := d.GetTopFilesWithCounts(10)
	if err != nil {
		t.Fatalf("GetTopFilesWithCounts: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	// a.go has 3 symbols → should be first.
	if results[0].Path != "a.go" {
		t.Errorf("want first=a.go, got %q", results[0].Path)
	}
	if results[0].Count != 3 {
		t.Errorf("want count=3, got %d", results[0].Count)
	}
	if results[1].Path != "b.go" || results[1].Count != 1 {
		t.Errorf("unexpected second result: %+v", results[1])
	}
}
```

- [ ] **Step 2: Run test — verify it fails**

```bash
go test -tags fts5 ./internal/db/ -run TestGetTopFilesWithCounts -v
```

Expected: FAIL — `GetTopFilesWithCounts` not defined.

- [ ] **Step 3: Add FileCount type and GetTopFilesWithCounts to db.go**

In `internal/db/db.go`, after the `GetTopFiles` function (around line 375), add:

```go
// FileCount pairs a file path with its symbol count.
type FileCount struct {
	Path  string
	Count int
}

// GetTopFilesWithCounts returns the top files by symbol count, descending, with counts.
func (d *Database) GetTopFilesWithCounts(limit int) ([]FileCount, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rows, err := d.db.Query(
		`SELECT file_path, COUNT(*) AS cnt FROM symbols GROUP BY file_path ORDER BY cnt DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []FileCount
	for rows.Next() {
		var fc FileCount
		if err := rows.Scan(&fc.Path, &fc.Count); err != nil {
			return nil, err
		}
		result = append(result, fc)
	}
	return result, rows.Err()
}
```

- [ ] **Step 4: Run test — verify it passes**

```bash
go test -tags fts5 ./internal/db/ -run TestGetTopFilesWithCounts -v
```

Expected: PASS.

- [ ] **Step 5: Rewrite cmdStatus in main.go**

In `cmd/code-outline-graph/main.go`, replace the entire body of `cmdStatus` after `openProjectDB` with:

```go
func cmdStatus(args []string) {
	positional, _ := parseFlags(args)

	rawPath := "."
	if len(positional) > 0 {
		rawPath = positional[0]
	}

	projectPath, database := openProjectDB(rawPath)
	defer database.Close()
	dbPath := paths.ProjectDBPath(projectPath)

	fileCount, symbolCount, err := database.GetFileStats()
	if err != nil {
		errorf("status error: %v", err)
	}

	topFiles, err := database.GetTopFilesWithCounts(10)
	if err != nil {
		errorf("status error: %v", err)
	}

	fmt.Printf("%sProject:%s  %s%s%s\n", colorBold, colorReset, colorCyan, projectPath, colorReset)
	fmt.Printf("%sDatabase:%s %s%s%s\n", colorBold, colorReset, colorDim, dbPath, colorReset)
	fmt.Printf("%sFiles indexed:%s  %s%d%s\n", colorBold, colorReset, colorGreen, fileCount, colorReset)
	fmt.Printf("%sTotal symbols:%s  %s%d%s\n", colorBold, colorReset, colorGreen, symbolCount, colorReset)

	if len(topFiles) > 0 {
		fmt.Printf("\n%sTop files by symbol count:%s\n", colorBold, colorReset)
		for _, f := range topFiles {
			fmt.Printf("  %s%-50s%s %s(%d symbols)%s\n",
				colorCyan, f.Path, colorReset,
				colorDim, f.Count, colorReset)
		}
	}
}
```

Also remove the now-unused `"sort"` import and the `fileStat` local type if they appear in `cmdStatus` (they were local to the old function, so they should be gone). Remove the unused `search` package import from `cmdStatus` if it's no longer needed by any other function in the file — check before removing.

- [ ] **Step 6: Build and smoke test**

```bash
go build -buildvcs=false -tags fts5 -o code-outline-graph ./cmd/code-outline-graph/
./code-outline-graph status .
```

Expected: builds and prints status without error.

- [ ] **Step 7: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go cmd/code-outline-graph/main.go
git commit -m "fix(status): replace O(n²) full-table scan with SQL aggregation

cmdStatus was doing LIKE '%%' scan + bubble sort over up to 10k rows.
Now uses GetFileStats() (two COUNT(*) queries) and GetTopFilesWithCounts()
(one GROUP BY query). Adds FileCount type to db package.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 3: Implement --force flag in indexer

`parseFlags` accepts `--force` but `_ = f.force` in both `cmdBuild` and `cmdUpdate` discards it. Adding `Force bool` to `Indexer` and bypassing the freshness fast-path when true gives the flag effect.

**Files:**
- Modify: `internal/indexer/indexer.go`
- Create: `internal/indexer/indexer_test.go`
- Modify: `cmd/code-outline-graph/main.go`

- [ ] **Step 1: Write failing behavioral test**

Create `internal/indexer/indexer_test.go`:

```go
package indexer_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gocode-outline-graph/internal/db"
	"gocode-outline-graph/internal/indexer"
)

func openTestDB(t *testing.T) *db.Database {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "index.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

// TestForce_reindexesDespiteFreshnessMatch verifies that Force=true causes
// re-indexing even when the file appears current (mtime + size unchanged).
//
// Strategy: index a file, then overwrite it with same-length content and
// restore the original mtime via os.Chtimes so IsFileCurrent returns true.
// Without Force the new symbol is invisible; with Force it appears.
func TestForce_reindexesDespiteFreshnessMatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "pkg.go")

	// v1: func Hello
	v1 := []byte("package pkg\n\nfunc Hello() {}\n")
	// v2: func World — same byte length as v1
	v2 := []byte("package pkg\n\nfunc World() {}\n")
	if len(v1) != len(v2) {
		t.Fatal("test setup: v1 and v2 must have identical byte lengths")
	}

	if err := os.WriteFile(filePath, v1, 0644); err != nil {
		t.Fatal(err)
	}

	database := openTestDB(t)
	idx := indexer.New(database, 1)

	// First index: Hello should appear.
	if _, err := idx.IndexAll(dir); err != nil {
		t.Fatalf("IndexAll v1: %v", err)
	}
	syms, _ := database.GetSymbolsForFile(filePath)
	if len(syms) == 0 || syms[0].Name != "Hello" {
		t.Fatalf("expected Hello after v1 index, got %v", syms)
	}

	// Capture original mtime.
	fi, err := os.Stat(filePath)
	if err != nil {
		t.Fatal(err)
	}
	origMtime := fi.ModTime()

	// Overwrite with v2 (same size), then restore original mtime.
	if err := os.WriteFile(filePath, v2, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filePath, origMtime, origMtime); err != nil {
		t.Fatal(err)
	}

	// Without Force: IsFileCurrent returns true → Hello still in DB.
	idx2 := indexer.New(database, 1)
	idx2.Force = false
	if _, err := idx2.IndexAll(dir); err != nil {
		t.Fatalf("IndexAll no-force: %v", err)
	}
	syms, _ = database.GetSymbolsForFile(filePath)
	if len(syms) == 0 || syms[0].Name != "Hello" {
		t.Fatalf("expected Hello still present (no force), got %v", syms)
	}

	// With Force: bypasses IsFileCurrent → World should appear.
	idx3 := indexer.New(database, 1)
	idx3.Force = true
	if _, err := idx3.IndexAll(dir); err != nil {
		t.Fatalf("IndexAll force: %v", err)
	}
	syms, _ = database.GetSymbolsForFile(filePath)
	found := false
	for _, s := range syms {
		if s.Name == "World" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected World after force re-index, got %v", syms)
	}

	// Suppress unused import.
	_ = time.Now()
}
```

- [ ] **Step 2: Run test — verify it fails**

```bash
go test -tags fts5 ./internal/indexer/ -run TestForce -v
```

Expected: FAIL — `idx.Force` field undefined.

- [ ] **Step 3: Add Force field to Indexer and wire into fast-path checks**

In `internal/indexer/indexer.go`, update the `Indexer` struct:

```go
// Indexer orchestrates project and single-file indexing.
type Indexer struct {
	database   *db.Database
	workers    int
	Force      bool // if true, skip freshness and checksum fast-paths
	OnProgress func(done, total int, filePath string)
}
```

In `indexFileWithParser`, update the two skip conditions:

```go
// Fast-path: skip if file hasn't changed (unless Force is set).
if !idx.Force && idx.database.IsFileCurrent(filePath, size, mtimeNs) {
	return nil
}

data, err := os.ReadFile(filePath)
if err != nil {
	return err
}

checksum := checksumBytes(data)

// Double-check with checksum in case mtime was bumped without content change.
if !idx.Force {
	existing := idx.database.GetChecksumForFile(filePath)
	if existing == checksum {
		return nil
	}
}
```

- [ ] **Step 4: Run test — verify it passes**

```bash
go test -tags fts5 ./internal/indexer/ -run TestForce -v
```

Expected: PASS.

- [ ] **Step 5: Wire idx.Force = f.force in main.go**

In `cmdBuild`, after `idx := indexer.New(database, f.workers)`, add:

```go
idx.Force = f.force
```

In `cmdUpdate`, after `idx := indexer.New(database, f.workers)`, add:

```go
idx.Force = f.force
```

Remove the `_ = f.force` comment line from `cmdUpdate`.

- [ ] **Step 6: Build and smoke test**

```bash
go build -buildvcs=false -tags fts5 -o code-outline-graph ./cmd/code-outline-graph/
./code-outline-graph build . --force
```

Expected: builds, `--force` causes full re-index of all files.

- [ ] **Step 7: Commit**

```bash
git add internal/indexer/indexer.go internal/indexer/indexer_test.go cmd/code-outline-graph/main.go
git commit -m "feat(indexer): implement --force flag to bypass freshness fast-path

Force=true skips IsFileCurrent and checksum double-check, always
re-parsing. Wires f.force into both cmdBuild and cmdUpdate.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Fix find_callees — resolve callee names to symbols with file:line info

`GetCallees` returns `[]string` (bare names). An LLM agent can't navigate from a bare name. `GetCalleeSymbols` JOINs `call_graph → symbols` and returns resolved Symbol objects plus a list of unresolved (external/stdlib) names.

**Files:**
- Modify: `internal/db/db.go`
- Modify: `internal/db/db_test.go` (add test)
- Modify: `internal/server/tools.go` (update handleFindCallees)

- [ ] **Step 1: Write failing test for GetCalleeSymbols**

Append to `internal/db/db_test.go`:

```go
func TestGetCalleeSymbols(t *testing.T) {
	d := openTestDB(t)

	// Insert caller and callee symbols.
	insertSymbols(t, d, "caller.go", []db.Symbol{
		{Name: "MyFunc", Kind: "function", Language: "go", StartLine: 1, EndLine: 5,
			Calls: []string{"HelperA", "externalPkg.Thing"}},
	})
	insertSymbols(t, d, "helper.go", []db.Symbol{
		{Name: "HelperA", Kind: "function", Language: "go", StartLine: 1, EndLine: 3},
	})

	caller, err := d.GetSymbolByName("MyFunc", "caller.go")
	if err != nil || caller == nil {
		t.Fatalf("get caller: %v", err)
	}

	resolved, unresolved, err := d.GetCalleeSymbols(caller.ID)
	if err != nil {
		t.Fatalf("GetCalleeSymbols: %v", err)
	}

	// HelperA is in the index → resolved.
	if len(resolved) != 1 || resolved[0].Name != "HelperA" {
		t.Errorf("want 1 resolved (HelperA), got %v", resolved)
	}

	// externalPkg.Thing is not in index → unresolved.
	if len(unresolved) != 1 || unresolved[0] != "externalPkg.Thing" {
		t.Errorf("want 1 unresolved (externalPkg.Thing), got %v", unresolved)
	}
}
```

- [ ] **Step 2: Run test — verify it fails**

```bash
go test -tags fts5 ./internal/db/ -run TestGetCalleeSymbols -v
```

Expected: FAIL — `GetCalleeSymbols` undefined.

- [ ] **Step 3: Add GetCalleeSymbols to db.go**

In `internal/db/db.go`, after `GetCallees` (around line 397), add:

```go
// GetCalleeSymbols returns resolved symbols and unresolved callee names for callerID.
// Resolved: callees whose name matches a symbol in the index.
// Unresolved: callee names with no matching symbol (external/stdlib).
func (d *Database) GetCalleeSymbols(callerID int64) (resolved []Symbol, unresolved []string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Resolved: JOIN call_graph → symbols on name.
	rows, err := d.db.Query(`
		SELECT DISTINCT s.`+symbolCols+`
		FROM call_graph cg
		JOIN symbols s ON s.name = cg.callee_name
		WHERE cg.caller_id = ?
		ORDER BY s.file_path, s.start_line`, callerID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	resolved, err = collectSymbols(rows)
	if err != nil {
		return nil, nil, err
	}

	// Build set of resolved names to find unresolved ones.
	resolvedNames := make(map[string]bool, len(resolved))
	for _, s := range resolved {
		resolvedNames[s.Name] = true
	}

	// Fetch all callee names for this caller.
	nameRows, err := d.db.Query(
		`SELECT callee_name FROM call_graph WHERE caller_id = ? ORDER BY callee_name`, callerID)
	if err != nil {
		return nil, nil, err
	}
	defer nameRows.Close()
	for nameRows.Next() {
		var n string
		if err := nameRows.Scan(&n); err != nil {
			return nil, nil, err
		}
		if !resolvedNames[n] {
			unresolved = append(unresolved, n)
		}
	}
	return resolved, unresolved, nameRows.Err()
}
```

- [ ] **Step 4: Run test — verify it passes**

```bash
go test -tags fts5 ./internal/db/ -run TestGetCalleeSymbols -v
```

Expected: PASS.

- [ ] **Step 5: Update handleFindCallees in tools.go**

Replace the body of `handleFindCallees` in `internal/server/tools.go` from the `callees, err := database.GetCallees(sym.ID)` line through the end of the function:

```go
	resolved, unresolved, err := database.GetCalleeSymbols(sym.ID)
	if err != nil {
		return toolError("get callees: %v", err)
	}

	if len(resolved) == 0 && len(unresolved) == 0 {
		return textResult(fmt.Sprintf("No callees found for %q", args.SymbolName))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Callees of %q:\n", args.SymbolName)
	for _, s := range resolved {
		fmt.Fprintf(&sb, "  %s:%d %s %s\n", s.FilePath, s.StartLine, s.Kind, s.Name)
		if s.Signature != "" {
			fmt.Fprintf(&sb, "    %s\n", s.Signature)
		}
	}
	for _, name := range unresolved {
		fmt.Fprintf(&sb, "  (unresolved) %s\n", name)
	}
	return textResult(sb.String())
```

- [ ] **Step 6: Build**

```bash
go build -buildvcs=false -tags fts5 -o code-outline-graph ./cmd/code-outline-graph/
```

Expected: exits 0.

- [ ] **Step 7: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go internal/server/tools.go
git commit -m "fix(tools): resolve callee names to file:line symbols in find_callees

GetCalleeSymbols JOINs call_graph → symbols to return full Symbol
objects for internal callees. External/stdlib names not in the index
are listed as '(unresolved)'. Matches find_callers output format.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 5: Fix watcher — watch newly created subdirectories

`Start()` adds only directories existing at startup. A new directory created afterward is never watched — files inside it are silently ignored.

**Files:**
- Modify: `internal/watcher/watcher.go`

Note: fsnotify directory watching is integration-level behavior. This task has no unit test — verify manually by running `serve` with `index_project(watch=true)` and creating a new subdirectory.

- [ ] **Step 1: Update handleEvent to detect new directories**

In `internal/watcher/watcher.go`, update `handleEvent` to check for new directories before the `IsSupported` guard:

```go
func (w *CodeWatcher) handleEvent(event fsnotify.Event) {
	path := event.Name
	op := event.Op

	// If a new directory was created, add it to the watcher immediately.
	if op&fsnotify.Create != 0 {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			_ = w.watcher.Add(path)
			return
		}
	}

	// Only process files we can parse.
	if !parser.IsSupported(path) {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Cancel any existing pending timer for this path.
	if t, exists := w.debounce[path]; exists {
		t.Stop()
	}

	w.debounce[path] = time.AfterFunc(debounceDuration, func() {
		w.mu.Lock()
		delete(w.debounce, path)
		w.mu.Unlock()

		w.dispatch(path, op)
	})
}
```

Add `"os"` to the imports in `watcher.go` if not already present.

- [ ] **Step 2: Build**

```bash
go build -buildvcs=false -tags fts5 -o code-outline-graph ./cmd/code-outline-graph/
```

Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
git add internal/watcher/watcher.go
git commit -m "fix(watcher): watch newly created subdirectories

Detect fsnotify.Create events on directories and add them to the
watcher immediately, so files inside new packages/modules are
auto-indexed without restarting the server.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Phase 2 — MCP Tool Enhancements

---

### Task 6: Add kind/language/file_pattern filters to find_by_keyword

New optional params on the existing `find_by_keyword` tool. Backed by a new `FilteredSearch` DB method. Zero changes to existing behavior when params are absent.

**Files:**
- Modify: `internal/db/db.go` (add FilteredSearch)
- Modify: `internal/db/db_test.go` (add test)
- Modify: `internal/server/tools.go` (update handler + schema)

- [ ] **Step 1: Write failing test for FilteredSearch**

Append to `internal/db/db_test.go`:

```go
func TestFilteredSearch_byKind(t *testing.T) {
	d := openTestDB(t)

	insertSymbols(t, d, "main.go", []db.Symbol{
		{Name: "RunServer", Kind: "function", Language: "go", StartLine: 1, EndLine: 5},
		{Name: "Config", Kind: "struct", Language: "go", StartLine: 10, EndLine: 20},
	})

	results, err := d.FilteredSearch("", "function", "", "", 20)
	if err != nil {
		t.Fatalf("FilteredSearch: %v", err)
	}
	if len(results) != 1 || results[0].Name != "RunServer" {
		t.Errorf("want [RunServer], got %v", results)
	}
}

func TestFilteredSearch_byLanguage(t *testing.T) {
	d := openTestDB(t)

	insertSymbols(t, d, "main.go", []db.Symbol{
		{Name: "GoFunc", Kind: "function", Language: "go"},
	})
	insertSymbols(t, d, "app.py", []db.Symbol{
		{Name: "py_func", Kind: "function", Language: "python"},
	})

	results, err := d.FilteredSearch("", "", "python", "", 20)
	if err != nil {
		t.Fatalf("FilteredSearch: %v", err)
	}
	if len(results) != 1 || results[0].Name != "py_func" {
		t.Errorf("want [py_func], got %v", results)
	}
}

func TestFilteredSearch_byFilePattern(t *testing.T) {
	d := openTestDB(t)

	insertSymbols(t, d, "/project/handlers/auth.go", []db.Symbol{
		{Name: "Login", Kind: "function", Language: "go"},
	})
	insertSymbols(t, d, "/project/models/user.go", []db.Symbol{
		{Name: "User", Kind: "struct", Language: "go"},
	})

	results, err := d.FilteredSearch("", "", "", "*/handlers/*", 20)
	if err != nil {
		t.Fatalf("FilteredSearch: %v", err)
	}
	if len(results) != 1 || results[0].Name != "Login" {
		t.Errorf("want [Login], got %v", results)
	}
}

func TestFilteredSearch_noFilters_delegatesToFTS(t *testing.T) {
	d := openTestDB(t)

	insertSymbols(t, d, "main.go", []db.Symbol{
		{Name: "HandleRequest", Kind: "function", Language: "go",
			Signature: "func HandleRequest(w http.ResponseWriter, r *http.Request)"},
	})

	results, err := d.FilteredSearch("HandleRequest", "", "", "", 20)
	if err != nil {
		t.Fatalf("FilteredSearch: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least one result for HandleRequest query")
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test -tags fts5 ./internal/db/ -run TestFilteredSearch -v
```

Expected: FAIL — `FilteredSearch` undefined.

- [ ] **Step 3: Add FilteredSearch to db.go**

In `internal/db/db.go`, after `FTSSearch` (around line 439), add:

```go
// FilteredSearch searches symbols by query string and optional kind/language/filePattern filters.
// When all filters are empty and query is set, delegates to FTS with keyword fallback.
// When any filter is set, runs filtered FTS then filtered keyword fallback.
// filePattern uses SQLite GLOB syntax (e.g. "*/handlers/*.go").
func (d *Database) FilteredSearch(query, kind, language, filePattern string, limit int) ([]Symbol, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	hasFilters := kind != "" || language != "" || filePattern != ""

	// FTS path (with or without filters).
	if query != "" {
		ftsQuery := buildFTSQuery(query)
		var rows *sql.Rows
		var err error
		if hasFilters {
			rows, err = d.db.Query(`
				SELECT s.`+symbolCols+`
				FROM symbols_fts fts
				JOIN symbols s ON s.id = fts.rowid
				WHERE symbols_fts MATCH ?
				AND (? = '' OR s.kind = ?)
				AND (? = '' OR s.language = ?)
				AND (? = '' OR s.file_path GLOB ?)
				ORDER BY rank
				LIMIT ?`,
				ftsQuery,
				kind, kind,
				language, language,
				filePattern, filePattern,
				limit)
		} else {
			rows, err = d.db.Query(`
				SELECT s.`+symbolCols+`
				FROM symbols_fts fts
				JOIN symbols s ON s.id = fts.rowid
				WHERE symbols_fts MATCH ?
				ORDER BY rank
				LIMIT ?`, ftsQuery, limit)
		}
		if err == nil {
			defer rows.Close()
			syms, serr := collectSymbols(rows)
			if serr == nil && len(syms) > 0 {
				return syms, nil
			}
		}
	}

	// Keyword LIKE fallback with optional filters.
	var clauses []string
	var qargs []interface{}

	if query != "" {
		pattern := "%" + escapeLike(query) + "%"
		clauses = append(clauses, `(name LIKE ? ESCAPE '\' OR signature LIKE ? ESCAPE '\')`)
		qargs = append(qargs, pattern, pattern)
	}
	if kind != "" {
		clauses = append(clauses, `kind = ?`)
		qargs = append(qargs, kind)
	}
	if language != "" {
		clauses = append(clauses, `language = ?`)
		qargs = append(qargs, language)
	}
	if filePattern != "" {
		clauses = append(clauses, `file_path GLOB ?`)
		qargs = append(qargs, filePattern)
	}

	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	qargs = append(qargs, limit)

	rows, err := d.db.Query(`SELECT `+symbolCols+` FROM symbols`+where+` LIMIT ?`, qargs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSymbols(rows)
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test -tags fts5 ./internal/db/ -run TestFilteredSearch -v
```

Expected: PASS all four.

- [ ] **Step 5: Update find_by_keyword tool schema in tools.go**

In `allTools()`, find the `find_by_keyword` entry and update its `InputSchema.Properties`:

```go
{
    Name:        "find_by_keyword",
    Description: "Full-text / keyword search across symbol names and signatures. Optional filters narrow by kind, language, or file glob.",
    InputSchema: ToolSchema{
        Type: "object",
        Properties: map[string]PropSchema{
            "query":        {Type: "string", Description: "Search query (leave empty to match all when using filters)"},
            "limit":        {Type: "integer", Description: "Maximum results to return (default 20)"},
            "project_path": {Type: "string", Description: "Project root"},
            "kind":         {Type: "string", Description: "Optional symbol kind filter: function, class, method, struct, interface, type, constant, module, enum, decorator"},
            "language":     {Type: "string", Description: "Optional language filter (e.g. 'go', 'python', 'typescript')"},
            "file_pattern": {Type: "string", Description: "Optional SQLite GLOB pattern for file_path (e.g. '*/handlers/*.go')"},
        },
        // query no longer Required — filters alone are valid (additive change, not breaking)
    },
},
```

- [ ] **Step 6: Update handleFindByKeyword in tools.go**

Replace the `args` struct and the search call in `handleFindByKeyword`:

```go
func handleFindByKeyword(raw json.RawMessage) interface{} {
	var args struct {
		Query       string `json:"query"`
		Limit       int    `json:"limit"`
		ProjectPath string `json:"project_path"`
		Kind        string `json:"kind"`
		Language    string `json:"language"`
		FilePattern string `json:"file_pattern"`
	}
	args.Limit = 20
	if raw != nil {
		json.Unmarshal(raw, &args) //nolint:errcheck
	}
	if args.Limit <= 0 {
		args.Limit = 20
	}

	if args.Query == "" && args.Kind == "" && args.Language == "" && args.FilePattern == "" {
		return toolError("query is required when no filters are specified")
	}

	projectPath := args.ProjectPath
	if projectPath == "" {
		return toolError("project_path is required")
	}

	database, _, _, err := GetComponents(projectPath)
	if err != nil {
		return toolError("init components: %v", err)
	}

	results, err := database.FilteredSearch(args.Query, args.Kind, args.Language, args.FilePattern, args.Limit)
	if err != nil {
		return toolError("search: %v", err)
	}

	if len(results) == 0 {
		return textResult(fmt.Sprintf("No symbols found for query: %q", args.Query))
	}

	var sb strings.Builder
	for _, s := range results {
		fmt.Fprintf(&sb, "%s:%d %s %s\n", s.FilePath, s.StartLine, s.Kind, s.Name)
		if s.Signature != "" {
			fmt.Fprintf(&sb, "  %s\n", s.Signature)
		}
	}
	return textResult(sb.String())
}
```

- [ ] **Step 7: Build**

```bash
go build -buildvcs=false -tags fts5 -o code-outline-graph ./cmd/code-outline-graph/
```

Expected: exits 0.

- [ ] **Step 8: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go internal/server/tools.go
git commit -m "feat(tools): add kind/language/file_pattern filters to find_by_keyword

FilteredSearch runs FTS with optional WHERE filters, falling back to
keyword LIKE search. Existing clients using find_by_keyword with only
'query' see identical behavior.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 7: Fix get_symbol — return all matches when file unspecified

`GetSymbolByName` returns `LIMIT 1` — silently picks an arbitrary match when multiple packages define the same name. `GetSymbolsByName` returns all matches. `handleGetSymbol` returns a JSON array only when `file_path` is unspecified and N > 1 (backward compat: single match still returns an object).

**Files:**
- Modify: `internal/db/db.go`
- Modify: `internal/db/db_test.go`
- Modify: `internal/server/tools.go`

- [ ] **Step 1: Write failing test for GetSymbolsByName**

Append to `internal/db/db_test.go`:

```go
func TestGetSymbolsByName_multipleFiles(t *testing.T) {
	d := openTestDB(t)

	insertSymbols(t, d, "pkg/a/main.go", []db.Symbol{
		{Name: "Handler", Kind: "function", Language: "go", StartLine: 1, EndLine: 5},
	})
	insertSymbols(t, d, "pkg/b/main.go", []db.Symbol{
		{Name: "Handler", Kind: "function", Language: "go", StartLine: 10, EndLine: 15},
	})

	// Without file_path: both results returned.
	all, err := d.GetSymbolsByName("Handler", "")
	if err != nil {
		t.Fatalf("GetSymbolsByName: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("want 2 results, got %d", len(all))
	}

	// With file_path: only that file's symbol.
	one, err := d.GetSymbolsByName("Handler", "pkg/a/main.go")
	if err != nil {
		t.Fatalf("GetSymbolsByName with file: %v", err)
	}
	if len(one) != 1 || one[0].FilePath != "pkg/a/main.go" {
		t.Errorf("want pkg/a/main.go symbol, got %v", one)
	}
}
```

- [ ] **Step 2: Run test — verify it fails**

```bash
go test -tags fts5 ./internal/db/ -run TestGetSymbolsByName -v
```

Expected: FAIL — `GetSymbolsByName` undefined.

- [ ] **Step 3: Add GetSymbolsByName to db.go**

In `internal/db/db.go`, after `GetSymbolByName` (around line 342), add:

```go
// GetSymbolsByName returns all symbols with the given name, optionally filtered by file.
func (d *Database) GetSymbolsByName(name, filePath string) ([]Symbol, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var rows *sql.Rows
	var err error
	if filePath != "" {
		rows, err = d.db.Query(
			`SELECT `+symbolCols+` FROM symbols WHERE name = ? AND file_path = ? ORDER BY start_line`,
			name, filePath)
	} else {
		rows, err = d.db.Query(
			`SELECT `+symbolCols+` FROM symbols WHERE name = ? ORDER BY file_path, start_line`,
			name)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSymbols(rows)
}
```

- [ ] **Step 4: Run test — verify it passes**

```bash
go test -tags fts5 ./internal/db/ -run TestGetSymbolsByName -v
```

Expected: PASS.

- [ ] **Step 5: Update handleGetSymbol in tools.go**

Replace the symbol lookup and response section of `handleGetSymbol` (from `sym, err := database.GetSymbolByName` through `return textResult(string(out))`):

```go
	syms, err := database.GetSymbolsByName(args.Name, filePath)
	if err != nil {
		return toolError("get symbol: %v", err)
	}
	if len(syms) == 0 {
		return textResult(fmt.Sprintf(`{"error":"not_found","name":%q}`, args.Name))
	}

	symToMap := func(s db.Symbol) map[string]interface{} {
		return map[string]interface{}{
			"name":       s.Name,
			"kind":       s.Kind,
			"file_path":  s.FilePath,
			"start_line": s.StartLine,
			"end_line":   s.EndLine,
			"signature":  s.Signature,
			"docstring":  s.Docstring,
			"parent":     s.Parent,
			"language":   s.Language,
		}
	}

	// Single match or file_path specified → return object (backward compat).
	if len(syms) == 1 || filePath != "" {
		out, _ := json.Marshal(symToMap(syms[0]))
		return textResult(string(out))
	}

	// Multiple matches → return array so the caller can disambiguate.
	var arr []map[string]interface{}
	for _, s := range syms {
		arr = append(arr, symToMap(s))
	}
	out, _ := json.Marshal(arr)
	return textResult(string(out))
```

Note: `db` package needs to be imported in `tools.go`. Check existing imports — if `db` is not imported directly (it's used via `GetComponents`), you may need to add the import or use a local variable. The `symToMap` closure captures `db.Symbol` via the parameter type — ensure `db` is imported: `"gocode-outline-graph/internal/db"`.

- [ ] **Step 6: Build**

```bash
go build -buildvcs=false -tags fts5 -o code-outline-graph ./cmd/code-outline-graph/
```

Expected: exits 0.

- [ ] **Step 7: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go internal/server/tools.go
git commit -m "feat(tools): get_symbol returns all matches when file_path unspecified

GetSymbolsByName replaces LIMIT 1 lookup. Single match or file_path
specified → returns JSON object (backward compat). Multiple matches →
returns JSON array for disambiguation.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 8: Add context_lines param to read_symbol_body

New optional param `context_lines` (default 0) expands the read range N lines before `start_line` and N lines after `end_line`. Useful when surrounding context matters (e.g. struct fields above a method).

**Files:**
- Modify: `internal/server/tools.go`

- [ ] **Step 1: Update handleReadSymbolBody**

In `internal/server/tools.go`, make two targeted edits to `handleReadSymbolBody`:

**Edit 1** — Add `ContextLines` to the `args` struct (the struct that starts after the func signature):

```go
	var args struct {
		Name         string `json:"name"`
		FilePath     string `json:"file_path"`
		ProjectPath  string `json:"project_path"`
		ContextLines int    `json:"context_lines"`
	}
```

**Edit 2** — Replace the final `readLinesNumbered` call and `return` (currently at the bottom of the function):

```go
	// Expand range by context_lines if requested.
	startLine := sym.StartLine - args.ContextLines
	if startLine < 1 {
		startLine = 1
	}
	endLine := sym.EndLine + args.ContextLines

	body, err := readLinesNumbered(targetFile, startLine, endLine)
	if err != nil {
		return toolError("read file: %v", err)
	}
	return textResult(body)
```

Everything else in `handleReadSymbolBody` stays unchanged.

Also update the tool schema in `allTools()` for `read_symbol_body`:

```go
"context_lines": {Type: "integer", Description: "Optional: expand read range by N lines before start and after end (default 0)"},
```

- [ ] **Step 2: Build**

```bash
go build -buildvcs=false -tags fts5 -o code-outline-graph ./cmd/code-outline-graph/
```

Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
git add internal/server/tools.go
git commit -m "feat(tools): add context_lines param to read_symbol_body

Expands the read range N lines before start_line and N lines after
end_line. Defaults to 0 (existing behavior unchanged).

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 9: Add list_files MCP tool

New tool that lists all indexed file paths, optionally filtered by a Go glob pattern. Backed by the existing `ListIndexedFiles()` DB method.

**Files:**
- Modify: `internal/server/tools.go`

- [ ] **Step 1: Add list_files schema to allTools()**

In `allTools()` in `internal/server/tools.go`, append:

```go
{
    Name:        "list_files",
    Description: "List all indexed file paths, optionally filtered by a glob pattern.",
    InputSchema: ToolSchema{
        Type: "object",
        Properties: map[string]PropSchema{
            "project_path": {Type: "string", Description: "Project root"},
            "file_pattern": {Type: "string", Description: "Optional glob pattern (e.g. '*.go', '*/handlers/*')"},
        },
        Required: []string{"project_path"},
    },
},
```

- [ ] **Step 2: Add handleListFiles function**

At the end of `internal/server/tools.go`, add:

```go
// 13. list_files
func handleListFiles(raw json.RawMessage) interface{} {
	var args struct {
		ProjectPath string `json:"project_path"`
		FilePattern string `json:"file_pattern"`
	}
	if raw != nil {
		json.Unmarshal(raw, &args) //nolint:errcheck
	}

	if args.ProjectPath == "" {
		return toolError("project_path is required")
	}

	database, _, _, err := GetComponents(args.ProjectPath)
	if err != nil {
		return toolError("init components: %v", err)
	}

	files, err := database.ListIndexedFiles()
	if err != nil {
		return toolError("list files: %v", err)
	}

	if args.FilePattern != "" {
		var filtered []string
		for _, f := range files {
			matched, matchErr := filepath.Match(args.FilePattern, f)
			if matchErr == nil && matched {
				filtered = append(filtered, f)
			}
			// Also try matching just the base name for patterns like "*.go".
			if !matched {
				if m, err2 := filepath.Match(args.FilePattern, filepath.Base(f)); err2 == nil && m {
					filtered = append(filtered, f)
				}
			}
		}
		files = filtered
	}

	if len(files) == 0 {
		return textResult("No indexed files found")
	}

	return textResult(strings.Join(files, "\n"))
}
```

- [ ] **Step 3: Register list_files in callTool switch**

In `callTool`'s switch statement, add before `default:`:

```go
case "list_files":
    return handleListFiles(p.Arguments)
```

- [ ] **Step 4: Build**

```bash
go build -buildvcs=false -tags fts5 -o code-outline-graph ./cmd/code-outline-graph/
```

Expected: exits 0.

- [ ] **Step 5: Commit**

```bash
git add internal/server/tools.go
git commit -m "feat(tools): add list_files MCP tool

Lists all indexed file paths. Optional file_pattern (Go filepath.Match
glob) filters by full path or base name. Backed by existing
ListIndexedFiles() — no new DB queries needed.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 10: Add find_by_kind MCP tool + GetSymbolsByKind DB method

New tool that returns all symbols of a given kind across the project. Hard limit 200 results.

**Files:**
- Modify: `internal/db/db.go`
- Modify: `internal/db/db_test.go`
- Modify: `internal/server/tools.go`

- [ ] **Step 1: Write failing test for GetSymbolsByKind**

Append to `internal/db/db_test.go`:

```go
func TestGetSymbolsByKind(t *testing.T) {
	d := openTestDB(t)

	insertSymbols(t, d, "main.go", []db.Symbol{
		{Name: "RunServer", Kind: "function", Language: "go", StartLine: 1, EndLine: 5},
		{Name: "Config", Kind: "struct", Language: "go", StartLine: 10, EndLine: 20},
		{Name: "NewConfig", Kind: "function", Language: "go", StartLine: 25, EndLine: 30},
	})

	fns, err := d.GetSymbolsByKind("function", "", 200)
	if err != nil {
		t.Fatalf("GetSymbolsByKind: %v", err)
	}
	if len(fns) != 2 {
		t.Errorf("want 2 functions, got %d", len(fns))
	}

	structs, err := d.GetSymbolsByKind("struct", "", 200)
	if err != nil {
		t.Fatalf("GetSymbolsByKind struct: %v", err)
	}
	if len(structs) != 1 || structs[0].Name != "Config" {
		t.Errorf("want [Config], got %v", structs)
	}
}

func TestGetSymbolsByKind_withLanguageFilter(t *testing.T) {
	d := openTestDB(t)

	insertSymbols(t, d, "main.go", []db.Symbol{
		{Name: "GoFunc", Kind: "function", Language: "go"},
	})
	insertSymbols(t, d, "app.py", []db.Symbol{
		{Name: "py_func", Kind: "function", Language: "python"},
	})

	results, err := d.GetSymbolsByKind("function", "go", 200)
	if err != nil {
		t.Fatalf("GetSymbolsByKind: %v", err)
	}
	if len(results) != 1 || results[0].Name != "GoFunc" {
		t.Errorf("want [GoFunc], got %v", results)
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test -tags fts5 ./internal/db/ -run TestGetSymbolsByKind -v
```

Expected: FAIL — `GetSymbolsByKind` undefined.

- [ ] **Step 3: Add GetSymbolsByKind to db.go**

In `internal/db/db.go`, after `GetTopFilesWithCounts` (added in Task 2), add:

```go
// GetSymbolsByKind returns all symbols of the given kind, optionally filtered by language.
// Results are ordered by file_path, start_line. Hard limit enforced by caller.
func (d *Database) GetSymbolsByKind(kind, language string, limit int) ([]Symbol, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var rows *sql.Rows
	var err error
	if language != "" {
		rows, err = d.db.Query(
			`SELECT `+symbolCols+` FROM symbols WHERE kind = ? AND language = ? ORDER BY file_path, start_line LIMIT ?`,
			kind, language, limit)
	} else {
		rows, err = d.db.Query(
			`SELECT `+symbolCols+` FROM symbols WHERE kind = ? ORDER BY file_path, start_line LIMIT ?`,
			kind, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSymbols(rows)
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test -tags fts5 ./internal/db/ -run TestGetSymbolsByKind -v
```

Expected: PASS both.

- [ ] **Step 5: Add find_by_kind schema to allTools()**

In `allTools()`, append:

```go
{
    Name:        "find_by_kind",
    Description: "Find all symbols of a given kind (function, class, method, struct, interface, type, constant, module, enum, decorator) across the project. Returns at most 200 results.",
    InputSchema: ToolSchema{
        Type: "object",
        Properties: map[string]PropSchema{
            "kind":         {Type: "string", Description: "Symbol kind: function, class, method, struct, interface, type, constant, module, enum, decorator"},
            "project_path": {Type: "string", Description: "Project root"},
            "language":     {Type: "string", Description: "Optional language filter (e.g. 'go', 'python')"},
        },
        Required: []string{"kind", "project_path"},
    },
},
```

- [ ] **Step 6: Add handleFindByKind function**

At the end of `internal/server/tools.go`, add:

```go
// 14. find_by_kind
func handleFindByKind(raw json.RawMessage) interface{} {
	var args struct {
		Kind        string `json:"kind"`
		ProjectPath string `json:"project_path"`
		Language    string `json:"language"`
	}
	if raw != nil {
		json.Unmarshal(raw, &args) //nolint:errcheck
	}

	if args.Kind == "" {
		return toolError("kind is required")
	}
	if args.ProjectPath == "" {
		return toolError("project_path is required")
	}

	database, _, _, err := GetComponents(args.ProjectPath)
	if err != nil {
		return toolError("init components: %v", err)
	}

	const maxResults = 200
	symbols, err := database.GetSymbolsByKind(args.Kind, args.Language, maxResults)
	if err != nil {
		return toolError("get symbols by kind: %v", err)
	}

	if len(symbols) == 0 {
		return textResult(fmt.Sprintf("No %q symbols found", args.Kind))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Symbols of kind %q (%d found):\n", args.Kind, len(symbols))
	for _, s := range symbols {
		fmt.Fprintf(&sb, "  %s:%d %s\n", s.FilePath, s.StartLine, s.Name)
		if s.Signature != "" {
			fmt.Fprintf(&sb, "    %s\n", s.Signature)
		}
	}
	return textResult(sb.String())
}
```

- [ ] **Step 7: Register find_by_kind in callTool switch**

```go
case "find_by_kind":
    return handleFindByKind(p.Arguments)
```

- [ ] **Step 8: Run all tests**

```bash
go test -tags fts5 ./...
```

Expected: all PASS.

- [ ] **Step 9: Build**

```bash
go build -buildvcs=false -tags fts5 -o code-outline-graph ./cmd/code-outline-graph/
```

Expected: exits 0.

- [ ] **Step 10: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go internal/server/tools.go
git commit -m "feat(tools): add find_by_kind MCP tool and GetSymbolsByKind DB method

Returns all symbols of a given kind (function, struct, class, etc.)
with optional language filter. Hard limit of 200 results. Useful for
'list all functions in this Go project' type queries.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Phase 3 — Language Extension Cleanup

---

### Task 11: Verify Phase 3 is complete

Phase 3 (trim `extensionMap`) was fully implemented in Task 1. No additional code changes needed.

- [ ] **Step 1: Verify extensionMap contains only 11 implemented grammars**

```bash
grep -n '^\s*"\.' internal/parser/languages.go
```

Expected output contains only `.py`, `.js`, `.mjs`, `.cjs`, `.ts`, `.tsx`, `.go`, `.rs`, `.java`, `.c`, `.h`, `.cpp`, `.cc`, `.cxx`, `.hpp`, `.hh`, `.hxx`, `.rb`, `.sh`, `.bash` — no `.json`, `.yaml`, `.html`, `.css`, `.md`, `.sql`, `.vue`, `.svelte`, `.lua`, `.cs`, `.kt`, `.swift`, `.php`, `.dart`, `.zig`, `.toml`, `.scala`.

- [ ] **Step 2: Run all tests one final time**

```bash
go test -tags fts5 ./...
```

Expected: all PASS.

- [ ] **Step 3: Final build**

```bash
go build -buildvcs=false -tags fts5 -o code-outline-graph ./cmd/code-outline-graph/
```

Expected: exits 0.

---

## Self-Review Checklist

| Spec Requirement | Covered By |
|-----------------|------------|
| Fix IsSupported (broken extensions) | Task 1 |
| Fix cmdStatus O(n²) + full-table scan | Task 2 |
| Implement --force flag | Task 3 |
| Fix find_callees to return file:line | Task 4 |
| Fix watcher for new directories | Task 5 |
| find_by_keyword kind/language/file_pattern filters | Task 6 |
| get_symbol multi-match return | Task 7 |
| read_symbol_body context_lines | Task 8 |
| list_files tool | Task 9 |
| find_by_kind tool | Task 10 |
| Phase 3 language cleanup | Task 1 (combined) |
