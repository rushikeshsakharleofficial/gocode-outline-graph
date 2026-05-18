# CLAUDE.md

Guidance for Claude Code when working in this repository.

## Overview

Go port of `code-outline-graph` — a source-code indexing tool that parses files with tree-sitter,
stores extracted symbols in SQLite (FTS5), and exposes the index as an MCP server over stdio.

Dropped from Python version: fastembed embeddings, sqlite-vec vector search.

## Build

**Always build with `-tags fts5`** — FTS5 is not enabled in go-sqlite3 by default:

```bash
go build -buildvcs=false -tags fts5 -o code-outline-graph-go ./cmd/code-outline-graph/

# Or via Makefile:
make build
make install   # installs to $GOPATH/bin
```

## Commands

```bash
code-outline-graph-go build .         # full index
code-outline-graph-go update .        # incremental reindex
code-outline-graph-go search . query  # FTS5 search
code-outline-graph-go outline . file  # list symbols for file
code-outline-graph-go status .        # DB stats
code-outline-graph-go serve           # start MCP server (stdio)
code-outline-graph-go prune .         # remove stale entries
code-outline-graph-go install         # write MCP config for editors
code-outline-graph-go version         # print version
```

## Architecture

Dependency order (no import cycles):

```
paths → db → parser → indexer → search/watcher → server → main
```

| Package | Role |
|---|---|
| `internal/paths` | Resolve project root; compute `.code-outline-graph/index.db` path |
| `internal/db` | SQLite schema, CRUD, FTS5 search, call graph, freshness checks |
| `internal/parser` | tree-sitter AST walking → Symbol extraction (11 languages) |
| `internal/indexer` | Goroutine worker pool, gitignore, blake2b checksums, freshness |
| `internal/search` | FTS5 + keyword search wrappers, RRF merge for ResolveEditTarget |
| `internal/watcher` | fsnotify file watcher, 500ms debounce, incremental reindex |
| `internal/server` | Hand-rolled JSON-RPC 2.0 over stdio; 12 MCP tools |
| `cmd/code-outline-graph` | CLI: build/update/search/outline/status/serve/prune/install |

## Key Dependencies

- `github.com/mattn/go-sqlite3` — CGo SQLite driver; FTS5 enabled via `-tags fts5`
- `github.com/smacker/go-tree-sitter` — Go bindings for tree-sitter C library
- `github.com/fsnotify/fsnotify` — cross-platform file watching

## Supported Languages

python, javascript, typescript, tsx, go, rust, java, c, cpp, ruby, bash
(languages.go maps 30+ extensions to these 11 tree-sitter grammars)

## Database

- Location: `<project-root>/.code-outline-graph/index.db`
- WAL mode, 64 MB page cache
- Tables: `symbols`, `symbols_fts` (FTS5 content table), `indexed_files`, `call_graph`
- FTS5 auto-synced via INSERT/UPDATE/DELETE triggers on `symbols`

## MCP Protocol

Hand-rolled JSON-RPC 2.0 over stdin/stdout (no external MCP SDK).
Newline-delimited JSON. 14 MCP tools.

## After Code Changes

```bash
go build -buildvcs=false -tags fts5 -o code-outline-graph-go ./cmd/code-outline-graph/
./code-outline-graph-go update .
```
