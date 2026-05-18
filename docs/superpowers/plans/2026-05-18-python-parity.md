# Python Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring the Go code-outline-graph to full feature parity with the Python production version — 40+ languages, 4 missing CLI commands, and 1 missing MCP tool.

**Architecture:** Split the monolithic `parser.go` into focused walk files by language family. All new tree-sitter grammars come from the already-vendored `github.com/smacker/go-tree-sitter` package (no new module dependencies). Languages without Go tree-sitter bindings get a line-based fallback walker producing `section` symbols.

**Tech Stack:** Go 1.25, github.com/smacker/go-tree-sitter, go-sqlite3 (fts5), encoding/json (for JSON files)

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `internal/parser/languages.go` | Modify | Add 50+ new extension/basename mappings |
| `internal/parser/walk_systems.go` | Create | C#, PHP, Swift, Kotlin, Scala walkers |
| `internal/parser/walk_config.go` | Create | YAML, TOML, HCL, Protobuf, JSON walkers |
| `internal/parser/walk_web.go` | Create | HTML, CSS, Svelte walkers |
| `internal/parser/walk_scripting.go` | Create | Lua, Elixir, Groovy, OCaml walkers |
| `internal/parser/walk_markup.go` | Create | Markdown, SQL, Dockerfile walkers |
| `internal/parser/walk_fallback.go` | Create | Line-based fallback for 15+ languages |
| `internal/parser/parser.go` | Modify | Add new parsers to `New()`, new cases in `Parse()` |
| `internal/parser/languages_test.go` | Modify | Accept new extensions, remove rejections |
| `internal/server/tools.go` | Modify | Add `resolve_edit_target` tool |
| `cmd/code-outline-graph/main.go` | Modify | Add `doctor`, `export`, `callers`, `callees` commands |

---

## Task 1: Expand languages.go

**Files:**
- Modify: `internal/parser/languages.go`
- Modify: `internal/parser/languages_test.go`

- [ ] **Step 1: Replace extensionMap and basenameMap in `internal/parser/languages.go`**

Replace the entire `extensionMap` and `basenameMap` variable blocks with:

```go
var extensionMap = map[string]string{
	// Already supported
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
	// Systems languages
	".cs":   "csharp",
	".php":  "php",
	".swift": "swift",
	".kt":   "kotlin",
	".kts":  "kotlin",
	".scala": "scala",
	// Config / data
	".yaml": "yaml",
	".yml":  "yaml",
	".toml": "toml",
	".tf":   "hcl",
	".hcl":  "hcl",
	".proto": "proto",
	".json": "json",
	// Web
	".html": "html",
	".htm":  "html",
	".css":  "css",
	".scss": "css",
	".sass": "css",
	".less": "css",
	".svelte": "svelte",
	".vue":  "svelte",
	// Scripting / functional
	".lua":  "lua",
	".ex":   "elixir",
	".exs":  "elixir",
	".groovy": "groovy",
	".gradle": "groovy",
	".ml":   "ocaml",
	".mli":  "ocaml",
	// Markup
	".md":   "markdown",
	".mdx":  "markdown",
	".sql":  "sql",
	// Fallback languages (line-based extraction)
	".dart": "dart",
	".zig":  "zig",
	".clj":  "clojure",
	".cljs": "clojure",
	".cljc": "clojure",
	".erl":  "erlang",
	".hrl":  "erlang",
	".hs":   "haskell",
	".lhs":  "haskell",
	".nix":  "nix",
	".fish": "fish",
	".pl":   "perl",
	".pm":   "perl",
	".t":    "perl",
	".r":    "r",
	".R":    "r",
	".ps1":  "powershell",
	".psm1": "powershell",
	".bat":  "batch",
	".cmd":  "batch",
	".graphql": "graphql",
	".gql":  "graphql",
	".xml":  "xml",
	".mk":   "make",
}

var basenameMap = map[string]string{
	"Dockerfile":         "dockerfile",
	"dockerfile":         "dockerfile",
	"Dockerfile.dev":     "dockerfile",
	"Dockerfile.prod":    "dockerfile",
	"Makefile":           "make",
	"makefile":           "make",
	"GNUmakefile":        "make",
}
```

- [ ] **Step 2: Update `internal/parser/languages_test.go` to match new support**

Replace the file contents:

```go
package parser_test

import (
	"testing"

	"gocode-outline-graph/internal/parser"
)

func TestIsSupported_acceptsImplementedLanguages(t *testing.T) {
	cases := []string{
		// existing
		"main.py", "index.js", "app.mjs", "mod.cjs",
		"types.ts", "App.tsx", "main.go", "lib.rs",
		"Hello.java", "lib.c", "util.h",
		"src.cpp", "src.cc", "src.cxx", "src.hpp", "src.hh", "src.hxx",
		"script.rb", "run.sh", "run.bash",
		// new tree-sitter
		"App.cs", "index.php", "App.swift", "Main.kt", "Main.kts", "Main.scala",
		"config.yaml", "config.yml", "config.toml", "main.tf", "infra.hcl",
		"api.proto", "data.json",
		"page.html", "page.htm", "style.css", "style.scss", "style.sass", "style.less",
		"App.svelte", "App.vue",
		"lib.lua", "module.ex", "config.exs", "build.groovy", "build.gradle",
		"lib.ml", "lib.mli",
		"README.md", "README.mdx", "schema.sql",
		// fallback languages
		"main.dart", "main.zig",
		"core.clj", "core.cljs", "core.cljc",
		"server.erl", "header.hrl",
		"Main.hs", "Lib.lhs",
		"config.nix", "config.fish",
		"script.pl", "Module.pm", "test.t",
		"analysis.r", "analysis.R",
		"deploy.ps1", "module.psm1",
		"build.bat", "run.cmd",
		"schema.graphql", "query.gql",
		"config.xml",
		"build.mk", "Makefile", "makefile",
		"Dockerfile", "Dockerfile.dev",
	}
	for _, f := range cases {
		if !parser.IsSupported(f) {
			t.Errorf("expected %q to be supported", f)
		}
	}
}

func TestIsSupported_rejectsUnknownExtensions(t *testing.T) {
	cases := []string{
		"binary.exe", "archive.zip", "image.png", "data.parquet",
		"noextension", ".hidden",
	}
	for _, f := range cases {
		if parser.IsSupported(f) {
			t.Errorf("expected %q to NOT be supported", f)
		}
	}
}
```

- [ ] **Step 3: Run tests — expect failures (walkers don't exist yet)**

```bash
cd /home/rushikesh.sakharle/Projects/gocode-outline-graph
go test -tags fts5 ./internal/parser/... 2>&1 | head -30
```

Expected: compilation errors because new language strings referenced but no walker added yet (that's fine — we'll fix in Task 8).

- [ ] **Step 4: Commit**

```bash
git add internal/parser/languages.go internal/parser/languages_test.go
git commit -m "feat(parser): expand language extension mappings to 40+ languages"
```

---

## Task 2: Create walk_systems.go (C#, PHP, Swift, Kotlin, Scala)

**Files:**
- Create: `internal/parser/walk_systems.go`

- [ ] **Step 1: Create the file**

```go
package parser

import (
	sitter "github.com/smacker/go-tree-sitter"

	"gocode-outline-graph/internal/db"
)

// ── C# ───────────────────────────────────────────────────────────────────────

func walkCSharp(node *sitter.Node, src []byte, filePath, parent string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	t := node.Type()
	switch t {
	case "namespace_declaration":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		syms = append(syms, sym(node, src, name, "module", filePath, parent, "csharp", "namespace "+name, "", nil))
		if body := node.ChildByFieldName("body"); body != nil {
			for i := 0; i < nchildren(body); i++ {
				syms = append(syms, walkCSharp(child(body, i), src, filePath, name)...)
			}
		}
	case "class_declaration":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		sig := "class " + name
		syms = append(syms, sym(node, src, name, "class", filePath, parent, "csharp", sig, "", nil))
		if body := node.ChildByFieldName("body"); body != nil {
			for i := 0; i < nchildren(body); i++ {
				syms = append(syms, walkCSharp(child(body, i), src, filePath, name)...)
			}
		}
	case "interface_declaration":
		name := field(node, "name", src)
		if name != "" {
			syms = append(syms, sym(node, src, name, "interface", filePath, parent, "csharp", "interface "+name, "", nil))
			if body := node.ChildByFieldName("body"); body != nil {
				for i := 0; i < nchildren(body); i++ {
					syms = append(syms, walkCSharp(child(body, i), src, filePath, name)...)
				}
			}
		}
	case "struct_declaration":
		name := field(node, "name", src)
		if name != "" {
			syms = append(syms, sym(node, src, name, "struct", filePath, parent, "csharp", "struct "+name, "", nil))
		}
	case "enum_declaration":
		name := field(node, "name", src)
		if name != "" {
			syms = append(syms, sym(node, src, name, "enum", filePath, parent, "csharp", "enum "+name, "", nil))
		}
	case "method_declaration", "constructor_declaration":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		params := field(node, "parameters", src)
		ret := field(node, "type", src)
		sig := ret + " " + name + params
		calls := collectCalls(node.ChildByFieldName("body"), src)
		kind := "method"
		syms = append(syms, sym(node, src, name, kind, filePath, parent, "csharp", sig, "", calls))
	case "property_declaration":
		name := field(node, "name", src)
		if name != "" {
			syms = append(syms, sym(node, src, name, "variable", filePath, parent, "csharp", name, "", nil))
		}
	default:
		for i := 0; i < nchildren(node); i++ {
			syms = append(syms, walkCSharp(child(node, i), src, filePath, parent)...)
		}
	}
	return syms
}

// ── PHP ───────────────────────────────────────────────────────────────────────

func walkPHP(node *sitter.Node, src []byte, filePath, parent string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	t := node.Type()
	switch t {
	case "namespace_definition":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		syms = append(syms, sym(node, src, name, "module", filePath, parent, "php", "namespace "+name, "", nil))
		if body := node.ChildByFieldName("body"); body != nil {
			for i := 0; i < nchildren(body); i++ {
				syms = append(syms, walkPHP(child(body, i), src, filePath, name)...)
			}
		}
	case "class_declaration":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		syms = append(syms, sym(node, src, name, "class", filePath, parent, "php", "class "+name, "", nil))
		if body := node.ChildByFieldName("body"); body != nil {
			for i := 0; i < nchildren(body); i++ {
				syms = append(syms, walkPHP(child(body, i), src, filePath, name)...)
			}
		}
	case "interface_declaration":
		name := field(node, "name", src)
		if name != "" {
			syms = append(syms, sym(node, src, name, "interface", filePath, parent, "php", "interface "+name, "", nil))
			if body := node.ChildByFieldName("body"); body != nil {
				for i := 0; i < nchildren(body); i++ {
					syms = append(syms, walkPHP(child(body, i), src, filePath, name)...)
				}
			}
		}
	case "trait_declaration":
		name := field(node, "name", src)
		if name != "" {
			syms = append(syms, sym(node, src, name, "interface", filePath, parent, "php", "trait "+name, "", nil))
		}
	case "enum_declaration":
		name := field(node, "name", src)
		if name != "" {
			syms = append(syms, sym(node, src, name, "enum", filePath, parent, "php", "enum "+name, "", nil))
		}
	case "function_definition":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		params := field(node, "parameters", src)
		calls := collectCalls(node.ChildByFieldName("body"), src)
		syms = append(syms, sym(node, src, name, "function", filePath, parent, "php", "function "+name+params, "", calls))
	case "method_declaration":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		params := field(node, "parameters", src)
		calls := collectCalls(node.ChildByFieldName("body"), src)
		syms = append(syms, sym(node, src, name, "method", filePath, parent, "php", "function "+name+params, "", calls))
	default:
		for i := 0; i < nchildren(node); i++ {
			syms = append(syms, walkPHP(child(node, i), src, filePath, parent)...)
		}
	}
	return syms
}

// ── Swift ────────────────────────────────────────────────────────────────────

func walkSwift(node *sitter.Node, src []byte, filePath, parent string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	t := node.Type()
	switch t {
	case "class_declaration":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		syms = append(syms, sym(node, src, name, "class", filePath, parent, "swift", "class "+name, "", nil))
		if body := node.ChildByFieldName("body"); body != nil {
			for i := 0; i < nchildren(body); i++ {
				syms = append(syms, walkSwift(child(body, i), src, filePath, name)...)
			}
		}
	case "struct_declaration":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		syms = append(syms, sym(node, src, name, "struct", filePath, parent, "swift", "struct "+name, "", nil))
		if body := node.ChildByFieldName("body"); body != nil {
			for i := 0; i < nchildren(body); i++ {
				syms = append(syms, walkSwift(child(body, i), src, filePath, name)...)
			}
		}
	case "protocol_declaration":
		name := field(node, "name", src)
		if name != "" {
			syms = append(syms, sym(node, src, name, "interface", filePath, parent, "swift", "protocol "+name, "", nil))
		}
	case "enum_declaration":
		name := field(node, "name", src)
		if name != "" {
			syms = append(syms, sym(node, src, name, "enum", filePath, parent, "swift", "enum "+name, "", nil))
		}
	case "extension_declaration":
		name := field(node, "type", src)
		if name == "" {
			// fallback: first identifier child
			for i := 0; i < nchildren(node); i++ {
				ch := child(node, i)
				if ch.Type() == "type_identifier" || ch.Type() == "user_type" {
					name = text(ch, src)
					break
				}
			}
		}
		if name != "" {
			syms = append(syms, sym(node, src, name, "class", filePath, parent, "swift", "extension "+name, "", nil))
			if body := node.ChildByFieldName("body"); body != nil {
				for i := 0; i < nchildren(body); i++ {
					syms = append(syms, walkSwift(child(body, i), src, filePath, name)...)
				}
			}
		}
	case "function_declaration":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		params := field(node, "params", src)
		sig := "func " + name + params
		calls := collectCalls(node.ChildByFieldName("body"), src)
		kind := "function"
		if parent != "" {
			kind = "method"
		}
		syms = append(syms, sym(node, src, name, kind, filePath, parent, "swift", sig, "", calls))
	case "init_declaration":
		calls := collectCalls(node.ChildByFieldName("body"), src)
		syms = append(syms, sym(node, src, "init", "method", filePath, parent, "swift", "init()", "", calls))
	default:
		for i := 0; i < nchildren(node); i++ {
			syms = append(syms, walkSwift(child(node, i), src, filePath, parent)...)
		}
	}
	return syms
}

// ── Kotlin ───────────────────────────────────────────────────────────────────

func walkKotlin(node *sitter.Node, src []byte, filePath, parent string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	t := node.Type()
	switch t {
	case "class_declaration":
		// Kotlin class name is in the first simple_identifier child
		name := kotlinSimpleName(node, src)
		if name == "" {
			break
		}
		syms = append(syms, sym(node, src, name, "class", filePath, parent, "kotlin", "class "+name, "", nil))
		if body := node.ChildByFieldName("class_body"); body != nil {
			for i := 0; i < nchildren(body); i++ {
				syms = append(syms, walkKotlin(child(body, i), src, filePath, name)...)
			}
		}
	case "object_declaration":
		name := kotlinSimpleName(node, src)
		if name != "" {
			syms = append(syms, sym(node, src, name, "class", filePath, parent, "kotlin", "object "+name, "", nil))
		}
	case "interface_declaration":
		name := kotlinSimpleName(node, src)
		if name != "" {
			syms = append(syms, sym(node, src, name, "interface", filePath, parent, "kotlin", "interface "+name, "", nil))
		}
	case "function_declaration":
		name := kotlinSimpleName(node, src)
		if name == "" {
			break
		}
		params := field(node, "function_value_parameters", src)
		sig := "fun " + name + params
		calls := collectCalls(node.ChildByFieldName("function_body"), src)
		kind := "function"
		if parent != "" {
			kind = "method"
		}
		syms = append(syms, sym(node, src, name, kind, filePath, parent, "kotlin", sig, "", calls))
	case "property_declaration":
		name := kotlinSimpleName(node, src)
		if name != "" {
			syms = append(syms, sym(node, src, name, "variable", filePath, parent, "kotlin", name, "", nil))
		}
	default:
		for i := 0; i < nchildren(node); i++ {
			syms = append(syms, walkKotlin(child(node, i), src, filePath, parent)...)
		}
	}
	return syms
}

// kotlinSimpleName finds the first simple_identifier in a node's named children.
func kotlinSimpleName(node *sitter.Node, src []byte) string {
	for i := 0; i < nnamed(node); i++ {
		ch := named(node, i)
		if ch.Type() == "simple_identifier" || ch.Type() == "type_identifier" {
			return text(ch, src)
		}
	}
	return ""
}

// ── Scala ────────────────────────────────────────────────────────────────────

func walkScala(node *sitter.Node, src []byte, filePath, parent string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	t := node.Type()
	switch t {
	case "class_definition":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		syms = append(syms, sym(node, src, name, "class", filePath, parent, "scala", "class "+name, "", nil))
		if body := node.ChildByFieldName("template_body"); body != nil {
			for i := 0; i < nchildren(body); i++ {
				syms = append(syms, walkScala(child(body, i), src, filePath, name)...)
			}
		}
	case "object_definition":
		name := field(node, "name", src)
		if name != "" {
			syms = append(syms, sym(node, src, name, "class", filePath, parent, "scala", "object "+name, "", nil))
		}
	case "trait_definition":
		name := field(node, "name", src)
		if name != "" {
			syms = append(syms, sym(node, src, name, "interface", filePath, parent, "scala", "trait "+name, "", nil))
		}
	case "function_definition":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		params := field(node, "parameters", src)
		sig := "def " + name + params
		calls := collectCalls(node.ChildByFieldName("body"), src)
		kind := "function"
		if parent != "" {
			kind = "method"
		}
		syms = append(syms, sym(node, src, name, kind, filePath, parent, "scala", sig, "", calls))
	case "val_definition":
		name := field(node, "pattern", src)
		if name != "" {
			syms = append(syms, sym(node, src, name, "constant", filePath, parent, "scala", "val "+name, "", nil))
		}
	default:
		for i := 0; i < nchildren(node); i++ {
			syms = append(syms, walkScala(child(node, i), src, filePath, parent)...)
		}
	}
	return syms
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/parser/walk_systems.go
git commit -m "feat(parser): add C#, PHP, Swift, Kotlin, Scala AST walkers"
```

---

## Task 3: Create walk_config.go (YAML, TOML, HCL, Protobuf, JSON)

**Files:**
- Create: `internal/parser/walk_config.go`

- [ ] **Step 1: Create `internal/parser/walk_config.go`**

```go
package parser

import (
	"encoding/json"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"gocode-outline-graph/internal/db"
)

// ── YAML ─────────────────────────────────────────────────────────────────────

func walkYAML(node *sitter.Node, src []byte, filePath string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	// Walk top-level block_mapping_pair nodes — each is a top-level key.
	var walk func(*sitter.Node, int)
	walk = func(n *sitter.Node, depth int) {
		if n == nil {
			return
		}
		if n.Type() == "block_mapping_pair" && depth <= 1 {
			keyNode := n.ChildByFieldName("key")
			if keyNode != nil {
				name := strings.TrimSpace(text(keyNode, src))
				if name != "" {
					syms = append(syms, sym(n, src, name, "section", filePath, "", "yaml", name+":", "", nil))
				}
			}
			return // don't recurse into nested mappings at depth 0
		}
		for i := 0; i < nchildren(n); i++ {
			walk(child(n, i), depth+1)
		}
	}
	walk(node, 0)
	return syms
}

// ── TOML ─────────────────────────────────────────────────────────────────────

func walkTOML(node *sitter.Node, src []byte, filePath string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	for i := 0; i < nchildren(node); i++ {
		ch := child(node, i)
		switch ch.Type() {
		case "table":
			// [section.name]
			name := text(ch, src)
			name = strings.Trim(strings.TrimSpace(name), "[]")
			if name != "" {
				syms = append(syms, sym(ch, src, name, "section", filePath, "", "toml", "["+name+"]", "", nil))
			}
		case "table_array_element":
			// [[array.name]]
			name := text(ch, src)
			name = strings.Trim(strings.TrimSpace(name), "[]")
			if name != "" {
				syms = append(syms, sym(ch, src, name, "section", filePath, "", "toml", "[["+name+"]]", "", nil))
			}
		}
	}
	return syms
}

// ── HCL ──────────────────────────────────────────────────────────────────────

func walkHCL(node *sitter.Node, src []byte, filePath string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	for i := 0; i < nchildren(node); i++ {
		ch := child(node, i)
		if ch.Type() != "block" {
			continue
		}
		// block: type label* body
		// e.g. resource "aws_instance" "web" { ... }
		parts := []string{}
		for j := 0; j < nnamed(ch); j++ {
			nc := named(ch, j)
			if nc.Type() == "identifier" || nc.Type() == "string_lit" {
				v := strings.Trim(text(nc, src), `"`)
				parts = append(parts, v)
			}
		}
		if len(parts) == 0 {
			continue
		}
		name := strings.Join(parts, ".")
		sig := strings.Join(parts, " ")
		syms = append(syms, sym(ch, src, name, "section", filePath, "", "hcl", sig, "", nil))
	}
	return syms
}

// ── Protobuf ─────────────────────────────────────────────────────────────────

func walkProto(node *sitter.Node, src []byte, filePath, parent string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	t := node.Type()
	switch t {
	case "message_definition":
		name := ""
		for i := 0; i < nnamed(node); i++ {
			ch := named(node, i)
			if ch.Type() == "message_name" || ch.Type() == "identifier" {
				name = text(ch, src)
				break
			}
		}
		if name != "" {
			syms = append(syms, sym(node, src, name, "class", filePath, parent, "proto", "message "+name, "", nil))
			if body := node.ChildByFieldName("body"); body != nil {
				for i := 0; i < nchildren(body); i++ {
					syms = append(syms, walkProto(child(body, i), src, filePath, name)...)
				}
			}
		}
	case "enum_definition":
		name := ""
		for i := 0; i < nnamed(node); i++ {
			ch := named(node, i)
			if ch.Type() == "enum_name" || ch.Type() == "identifier" {
				name = text(ch, src)
				break
			}
		}
		if name != "" {
			syms = append(syms, sym(node, src, name, "enum", filePath, parent, "proto", "enum "+name, "", nil))
		}
	case "service_definition":
		name := ""
		for i := 0; i < nnamed(node); i++ {
			ch := named(node, i)
			if ch.Type() == "service_name" || ch.Type() == "identifier" {
				name = text(ch, src)
				break
			}
		}
		if name != "" {
			syms = append(syms, sym(node, src, name, "interface", filePath, parent, "proto", "service "+name, "", nil))
			if body := node.ChildByFieldName("body"); body != nil {
				for i := 0; i < nchildren(body); i++ {
					syms = append(syms, walkProto(child(body, i), src, filePath, name)...)
				}
			}
		}
	case "rpc_definition":
		name := ""
		for i := 0; i < nnamed(node); i++ {
			ch := named(node, i)
			if ch.Type() == "rpc_name" || ch.Type() == "identifier" {
				name = text(ch, src)
				break
			}
		}
		if name != "" {
			syms = append(syms, sym(node, src, name, "function", filePath, parent, "proto", "rpc "+name, "", nil))
		}
	default:
		for i := 0; i < nchildren(node); i++ {
			syms = append(syms, walkProto(child(node, i), src, filePath, parent)...)
		}
	}
	return syms
}

// ── JSON (stdlib, no tree-sitter) ────────────────────────────────────────────

// walkJSONBytes extracts top-level keys from a JSON file as section symbols.
// Uses encoding/json since smacker/go-tree-sitter has no JSON grammar.
// Line numbers are approximated by scanning for the key in the source.
func walkJSONBytes(src []byte, filePath string) []db.Symbol {
	var top interface{}
	if err := json.Unmarshal(src, &top); err != nil {
		return nil
	}
	obj, ok := top.(map[string]interface{})
	if !ok {
		return nil
	}

	// Build a line-number index: scan for `"key":` occurrences.
	lines := strings.Split(string(src), "\n")
	lineForKey := func(key string) int {
		needle := `"` + key + `":`
		for i, line := range lines {
			if strings.Contains(line, needle) {
				return i + 1
			}
		}
		return 1
	}

	var syms []db.Symbol
	for key := range obj {
		lineNum := lineForKey(key)
		syms = append(syms, db.Symbol{
			Name:      key,
			Kind:      "section",
			FilePath:  filePath,
			StartLine: lineNum,
			EndLine:   lineNum,
			Signature: `"` + key + `":`,
			Language:  "json",
		})
	}
	return syms
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/parser/walk_config.go
git commit -m "feat(parser): add YAML, TOML, HCL, Protobuf, JSON walkers"
```

---

## Task 4: Create walk_web.go (HTML, CSS, Svelte)

**Files:**
- Create: `internal/parser/walk_web.go`

- [ ] **Step 1: Create `internal/parser/walk_web.go`**

```go
package parser

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"gocode-outline-graph/internal/db"
)

// ── HTML ─────────────────────────────────────────────────────────────────────

func walkHTML(node *sitter.Node, src []byte, filePath string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if n.Type() == "element" {
			tag := ""
			if start := n.ChildByFieldName("open_tag"); start != nil {
				if tagName := start.ChildByFieldName("tag_name"); tagName != nil {
					tag = strings.ToLower(text(tagName, src))
				} else {
					// fallback: first named child of open_tag
					for i := 0; i < nnamed(start); i++ {
						ch := named(start, i)
						if ch.Type() == "tag_name" {
							tag = strings.ToLower(text(ch, src))
							break
						}
					}
				}
			}
			switch tag {
			case "h1", "h2", "h3", "h4", "h5", "h6":
				// Use inner text as name.
				inner := strings.TrimSpace(innerText(n, src))
				if len(inner) > 80 {
					inner = inner[:80]
				}
				if inner == "" {
					inner = tag
				}
				syms = append(syms, sym(n, src, inner, "section", filePath, "", "html", "<"+tag+">"+inner+"</"+tag+">", "", nil))
			}
		}
		for i := 0; i < nchildren(n); i++ {
			walk(child(n, i))
		}
	}
	walk(node)
	return syms
}

// innerText extracts concatenated text content of an element node.
func innerText(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	var sb strings.Builder
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n.Type() == "text" {
			sb.WriteString(text(n, src))
		}
		for i := 0; i < nchildren(n); i++ {
			walk(child(n, i))
		}
	}
	walk(node)
	return sb.String()
}

// ── CSS ───────────────────────────────────────────────────────────────────────

func walkCSS(node *sitter.Node, src []byte, filePath string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		switch n.Type() {
		case "rule_set":
			// selector { ... }
			sel := n.ChildByFieldName("selectors")
			if sel == nil {
				// fallback: first child before "{"
				for i := 0; i < nchildren(n); i++ {
					ch := child(n, i)
					if ch.Type() != "{" {
						sel = ch
						break
					}
				}
			}
			if sel != nil {
				name := strings.TrimSpace(text(sel, src))
				if len(name) > 80 {
					name = name[:80]
				}
				if name != "" {
					syms = append(syms, sym(n, src, name, "section", filePath, "", "css", name+" { }", "", nil))
				}
			}
		case "media_statement":
			// @media (query) { ... }
			name := firstLine(n, src)
			if name != "" {
				syms = append(syms, sym(n, src, name, "section", filePath, "", "css", name, "", nil))
			}
		case "keyframes_statement":
			name := firstLine(n, src)
			if name != "" {
				syms = append(syms, sym(n, src, name, "section", filePath, "", "css", name, "", nil))
			}
		default:
			for i := 0; i < nchildren(n); i++ {
				walk(child(n, i))
			}
			return
		}
	}
	walk(node)
	return syms
}

// ── Svelte / Vue ─────────────────────────────────────────────────────────────

func walkSvelte(node *sitter.Node, src []byte, filePath string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	for i := 0; i < nchildren(node); i++ {
		ch := child(node, i)
		switch ch.Type() {
		case "script_element":
			syms = append(syms, sym(ch, src, "script", "section", filePath, "", "svelte", "<script>", "", nil))
		case "style_element":
			syms = append(syms, sym(ch, src, "style", "section", filePath, "", "svelte", "<style>", "", nil))
		}
	}
	return syms
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/parser/walk_web.go
git commit -m "feat(parser): add HTML, CSS, Svelte AST walkers"
```

---

## Task 5: Create walk_scripting.go (Lua, Elixir, Groovy, OCaml)

**Files:**
- Create: `internal/parser/walk_scripting.go`

- [ ] **Step 1: Create `internal/parser/walk_scripting.go`**

```go
package parser

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"gocode-outline-graph/internal/db"
)

// ── Lua ───────────────────────────────────────────────────────────────────────

func walkLua(node *sitter.Node, src []byte, filePath, parent string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	t := node.Type()
	switch t {
	case "function_declaration":
		name := field(node, "name", src)
		if name == "" {
			// Lua function name may be a dotted identifier
			for i := 0; i < nnamed(node); i++ {
				ch := named(node, i)
				if ch.Type() == "method_index_expression" || ch.Type() == "dot_index_expression" {
					name = text(ch, src)
					break
				}
				if ch.Type() == "identifier" {
					name = text(ch, src)
					break
				}
			}
		}
		if name == "" {
			break
		}
		params := field(node, "parameters", src)
		calls := collectCalls(node.ChildByFieldName("body"), src)
		kind := "function"
		if strings.Contains(name, ":") {
			kind = "method"
		}
		syms = append(syms, sym(node, src, name, kind, filePath, parent, "lua", "function "+name+params, "", calls))
	case "local_function":
		name := field(node, "name", src)
		if name == "" {
			for i := 0; i < nnamed(node); i++ {
				ch := named(node, i)
				if ch.Type() == "identifier" {
					name = text(ch, src)
					break
				}
			}
		}
		if name != "" {
			params := field(node, "parameters", src)
			calls := collectCalls(node.ChildByFieldName("body"), src)
			syms = append(syms, sym(node, src, name, "function", filePath, parent, "lua", "local function "+name+params, "", calls))
		}
	default:
		for i := 0; i < nchildren(node); i++ {
			syms = append(syms, walkLua(child(node, i), src, filePath, parent)...)
		}
	}
	return syms
}

// ── Elixir ───────────────────────────────────────────────────────────────────

func walkElixir(node *sitter.Node, src []byte, filePath, parent string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	t := node.Type()
	// Elixir is a call-based language; macros like def/defmodule look like calls.
	if t == "call" {
		funcNode := node.ChildByFieldName("target")
		if funcNode == nil && nnamed(node) > 0 {
			funcNode = named(node, 0)
		}
		if funcNode != nil {
			macroName := text(funcNode, src)
			switch macroName {
			case "defmodule":
				// defmodule ModuleName do ... end
				args := node.ChildByFieldName("arguments")
				if args != nil && nnamed(args) > 0 {
					modName := text(named(args, 0), src)
					modName = strings.TrimSpace(modName)
					if modName != "" {
						syms = append(syms, sym(node, src, modName, "module", filePath, parent, "elixir", "defmodule "+modName, "", nil))
						// Walk body (do block)
						doBlock := node.ChildByFieldName("do_block")
						if doBlock == nil {
							// look for do_block in children
							for i := 0; i < nchildren(node); i++ {
								ch := child(node, i)
								if ch.Type() == "do_block" {
									doBlock = ch
									break
								}
							}
						}
						if doBlock != nil {
							for i := 0; i < nchildren(doBlock); i++ {
								syms = append(syms, walkElixir(child(doBlock, i), src, filePath, modName)...)
							}
						}
					}
				}
			case "def", "defp", "defmacro", "defmacrop":
				args := node.ChildByFieldName("arguments")
				if args != nil && nnamed(args) > 0 {
					firstArg := named(args, 0)
					name := ""
					if firstArg.Type() == "call" {
						// def name(args), do: ...
						funcTarget := firstArg.ChildByFieldName("target")
						if funcTarget != nil {
							name = text(funcTarget, src)
						}
					} else {
						name = text(firstArg, src)
					}
					name = strings.Split(name, "(")[0]
					name = strings.TrimSpace(name)
					if name != "" {
						sig := macroName + " " + firstLine(node, src)
						calls := collectCalls(node.ChildByFieldName("do_block"), src)
						syms = append(syms, sym(node, src, name, "function", filePath, parent, "elixir", sig, "", calls))
					}
				}
			}
		}
		return syms
	}
	for i := 0; i < nchildren(node); i++ {
		syms = append(syms, walkElixir(child(node, i), src, filePath, parent)...)
	}
	return syms
}

// ── Groovy ───────────────────────────────────────────────────────────────────

func walkGroovy(node *sitter.Node, src []byte, filePath, parent string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	t := node.Type()
	switch t {
	case "class_declaration":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		syms = append(syms, sym(node, src, name, "class", filePath, parent, "groovy", "class "+name, "", nil))
		if body := node.ChildByFieldName("class_body"); body != nil {
			for i := 0; i < nchildren(body); i++ {
				syms = append(syms, walkGroovy(child(body, i), src, filePath, name)...)
			}
		}
	case "method_declaration":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		params := field(node, "formal_parameters", src)
		calls := collectCalls(node.ChildByFieldName("block"), src)
		kind := "function"
		if parent != "" {
			kind = "method"
		}
		syms = append(syms, sym(node, src, name, kind, filePath, parent, "groovy", name+params, "", calls))
	default:
		for i := 0; i < nchildren(node); i++ {
			syms = append(syms, walkGroovy(child(node, i), src, filePath, parent)...)
		}
	}
	return syms
}

// ── OCaml ────────────────────────────────────────────────────────────────────

func walkOCaml(node *sitter.Node, src []byte, filePath, parent string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	t := node.Type()
	switch t {
	case "module_definition":
		name := field(node, "name", src)
		if name == "" {
			for i := 0; i < nnamed(node); i++ {
				ch := named(node, i)
				if ch.Type() == "module_name" || ch.Type() == "upper_case_identifier" {
					name = text(ch, src)
					break
				}
			}
		}
		if name != "" {
			syms = append(syms, sym(node, src, name, "module", filePath, parent, "ocaml", "module "+name, "", nil))
			if body := node.ChildByFieldName("body"); body != nil {
				for i := 0; i < nchildren(body); i++ {
					syms = append(syms, walkOCaml(child(body, i), src, filePath, name)...)
				}
			}
		}
	case "let_binding":
		// Top-level let binding: let name = ...
		nameNode := node.ChildByFieldName("pattern")
		if nameNode == nil {
			for i := 0; i < nnamed(node); i++ {
				ch := named(node, i)
				if ch.Type() == "value_name" || ch.Type() == "lowercase_identifier" {
					nameNode = ch
					break
				}
			}
		}
		if nameNode != nil {
			name := text(nameNode, src)
			if name != "" {
				body := node.ChildByFieldName("body")
				kind := "constant"
				if body != nil && (body.Type() == "fun" || body.Type() == "function") {
					kind = "function"
				}
				syms = append(syms, sym(node, src, name, kind, filePath, parent, "ocaml", "let "+name, "", nil))
			}
		}
	case "type_definition":
		for i := 0; i < nnamed(node); i++ {
			ch := named(node, i)
			if ch.Type() == "type_binding" {
				name := field(ch, "name", src)
				if name == "" {
					for j := 0; j < nnamed(ch); j++ {
						nc := named(ch, j)
						if nc.Type() == "type_name" || nc.Type() == "lowercase_identifier" {
							name = text(nc, src)
							break
						}
					}
				}
				if name != "" {
					syms = append(syms, sym(ch, src, name, "type", filePath, parent, "ocaml", "type "+name, "", nil))
				}
			}
		}
	case "value_specification":
		// val name : type (in module signatures)
		name := ""
		for i := 0; i < nnamed(node); i++ {
			ch := named(node, i)
			if ch.Type() == "value_name" || ch.Type() == "lowercase_identifier" {
				name = text(ch, src)
				break
			}
		}
		if name != "" {
			syms = append(syms, sym(node, src, name, "function", filePath, parent, "ocaml", "val "+name, "", nil))
		}
	default:
		for i := 0; i < nchildren(node); i++ {
			syms = append(syms, walkOCaml(child(node, i), src, filePath, parent)...)
		}
	}
	return syms
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/parser/walk_scripting.go
git commit -m "feat(parser): add Lua, Elixir, Groovy, OCaml AST walkers"
```

---

## Task 6: Create walk_markup.go (Markdown, SQL, Dockerfile)

**Files:**
- Create: `internal/parser/walk_markup.go`

- [ ] **Step 1: Create `internal/parser/walk_markup.go`**

```go
package parser

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"gocode-outline-graph/internal/db"
)

// ── Markdown ─────────────────────────────────────────────────────────────────

func walkMarkdown(node *sitter.Node, src []byte, filePath string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		switch n.Type() {
		case "atx_heading":
			// atx_heading: # Heading text
			// The heading content is in atx_heading_content child
			name := ""
			for i := 0; i < nchildren(n); i++ {
				ch := child(n, i)
				if ch.Type() == "atx_heading_content" || ch.Type() == "inline" {
					name = strings.TrimSpace(text(ch, src))
					break
				}
			}
			if name == "" {
				// fallback: strip leading #
				raw := strings.TrimSpace(text(n, src))
				name = strings.TrimLeft(raw, "# ")
			}
			if len(name) > 100 {
				name = name[:100]
			}
			if name != "" {
				sig := strings.TrimSpace(firstLine(n, src))
				syms = append(syms, sym(n, src, name, "section", filePath, "", "markdown", sig, "", nil))
			}
		case "setext_heading":
			// setext_heading: Title\n===
			name := ""
			for i := 0; i < nchildren(n); i++ {
				ch := child(n, i)
				if ch.Type() == "paragraph" || ch.Type() == "inline" {
					name = strings.TrimSpace(text(ch, src))
					break
				}
			}
			if name != "" {
				syms = append(syms, sym(n, src, name, "section", filePath, "", "markdown", name, "", nil))
			}
		default:
			for i := 0; i < nchildren(n); i++ {
				walk(child(n, i))
			}
		}
	}
	walk(node)
	return syms
}

// ── SQL ───────────────────────────────────────────────────────────────────────

func walkSQL(node *sitter.Node, src []byte, filePath string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		t := n.Type()
		switch t {
		case "create_table_statement":
			name := sqlObjectName(n, src)
			if name != "" {
				syms = append(syms, sym(n, src, name, "table", filePath, "", "sql", "CREATE TABLE "+name, "", nil))
			}
		case "create_view_statement":
			name := sqlObjectName(n, src)
			if name != "" {
				syms = append(syms, sym(n, src, name, "view", filePath, "", "sql", "CREATE VIEW "+name, "", nil))
			}
		case "create_function_statement":
			name := sqlObjectName(n, src)
			if name != "" {
				syms = append(syms, sym(n, src, name, "function", filePath, "", "sql", "CREATE FUNCTION "+name, "", nil))
			}
		case "create_procedure_statement":
			name := sqlObjectName(n, src)
			if name != "" {
				syms = append(syms, sym(n, src, name, "function", filePath, "", "sql", "CREATE PROCEDURE "+name, "", nil))
			}
		default:
			for i := 0; i < nchildren(n); i++ {
				walk(child(n, i))
			}
		}
	}
	walk(node)
	return syms
}

// sqlObjectName finds the object name in a CREATE statement by looking for
// identifier or dotted_name children following the statement type keywords.
func sqlObjectName(node *sitter.Node, src []byte) string {
	for i := 0; i < nnamed(node); i++ {
		ch := named(node, i)
		switch ch.Type() {
		case "object_reference", "table_reference", "identifier", "dotted_name":
			return strings.TrimSpace(text(ch, src))
		}
	}
	return ""
}

// ── Dockerfile ───────────────────────────────────────────────────────────────

func walkDockerfile(node *sitter.Node, src []byte, filePath string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	stageNum := 0
	for i := 0; i < nchildren(node); i++ {
		ch := child(node, i)
		if ch.Type() == "from_instruction" {
			// FROM image AS alias
			name := ""
			// Check for AS alias
			for j := 0; j < nnamed(ch); j++ {
				nc := named(ch, j)
				if nc.Type() == "image_alias" || nc.Type() == "identifier" {
					name = text(nc, src)
				}
			}
			if name == "" {
				// Use the image name instead
				for j := 0; j < nnamed(ch); j++ {
					nc := named(ch, j)
					if nc.Type() == "image_spec" || nc.Type() == "string" {
						name = strings.Split(text(nc, src), ":")[0]
						name = strings.Split(name, "@")[0]
						break
					}
				}
			}
			if name == "" {
				stageNum++
				name = fmt.Sprintf("stage%d", stageNum)
			}
			sig := strings.TrimSpace(firstLine(ch, src))
			syms = append(syms, sym(ch, src, name, "section", filePath, "", "dockerfile", sig, "", nil))
		}
	}
	return syms
}
```

Note: `walk_markup.go` uses `fmt.Sprintf` so add `"fmt"` to the import block.

- [ ] **Step 2: Fix import — add "fmt" to walk_markup.go imports**

The imports in walk_markup.go should be:
```go
import (
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"gocode-outline-graph/internal/db"
)
```

- [ ] **Step 3: Commit**

```bash
git add internal/parser/walk_markup.go
git commit -m "feat(parser): add Markdown, SQL, Dockerfile AST walkers"
```

---

## Task 7: Create walk_fallback.go (line-based for 15+ languages)

**Files:**
- Create: `internal/parser/walk_fallback.go`

- [ ] **Step 1: Create `internal/parser/walk_fallback.go`**

```go
package parser

import (
	"bufio"
	"bytes"
	"strings"

	"gocode-outline-graph/internal/db"
)

// lineBasedPatterns maps language name to a list of keyword prefixes.
// When a source line starts with one of these prefixes (after trimming whitespace),
// that line becomes a "section" symbol.
var lineBasedPatterns = map[string][]string{
	"dart":       {"class ", "void ", "Future<", "Stream<", "Widget ", "abstract class ", "mixin ", "extension ", "enum "},
	"zig":        {"pub fn ", "fn ", "pub const ", "const ", "pub var ", "var ", "pub struct ", "pub enum ", "pub union ", "pub inline fn "},
	"clojure":    {"(defn ", "(defmacro ", "(ns ", "(def ", "(defrecord ", "(defprotocol ", "(deftype "},
	"erlang":     {"-module(", "-export(", "-spec "},
	"haskell":    {"data ", "newtype ", "type ", "class ", "instance "},
	"nix":        {"let ", "rec {", "in {"},
	"fish":       {"function "},
	"perl":       {"sub ", "package "},
	"r":          {" <- function", " = function", "setClass(", "setGeneric(", "setMethod("},
	"powershell": {"function ", "Filter ", "Class "},
	"batch":      {":"},
	"graphql":    {"type ", "input ", "enum ", "union ", "interface ", "scalar ", "directive "},
	"xml":        {"<"},
	"make":       {},
}

// walkLineBased produces section symbols by scanning source lines for language
// keyword markers. This is the fallback for languages without tree-sitter bindings.
func walkLineBased(src []byte, filePath, language string) []db.Symbol {
	patterns, ok := lineBasedPatterns[language]
	if !ok {
		return nil
	}

	var syms []db.Symbol
	scanner := bufio.NewScanner(bytes.NewReader(src))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "--") {
			continue
		}

		matched := false
		if language == "make" {
			// Makefile targets: non-indented lines ending with ":"
			if !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ") &&
				strings.Contains(trimmed, ":") && !strings.HasPrefix(trimmed, "#") {
				colonIdx := strings.Index(trimmed, ":")
				name := strings.TrimSpace(trimmed[:colonIdx])
				if name != "" && !strings.Contains(name, " ") {
					syms = append(syms, db.Symbol{
						Name:      name,
						Kind:      "section",
						FilePath:  filePath,
						StartLine: lineNum,
						EndLine:   lineNum,
						Signature: trimmed,
						Language:  language,
					})
				}
			}
			continue
		}

		if language == "batch" {
			// Batch labels: :labelname
			if strings.HasPrefix(trimmed, ":") && len(trimmed) > 1 && trimmed[1] != ':' {
				name := strings.TrimSpace(trimmed[1:])
				name = strings.Fields(name)[0]
				if name != "" {
					syms = append(syms, db.Symbol{
						Name:      name,
						Kind:      "section",
						FilePath:  filePath,
						StartLine: lineNum,
						EndLine:   lineNum,
						Signature: trimmed,
						Language:  language,
					})
				}
			}
			continue
		}

		for _, pat := range patterns {
			if strings.HasPrefix(trimmed, pat) {
				// Extract name: take the token after the keyword.
				rest := strings.TrimSpace(trimmed[len(pat):])
				name := ""
				// Split on common delimiters: ( { [ <space> =
				for _, sep := range []string{"(", "{", "[", " ", "=", ":"} {
					if idx := strings.Index(rest, sep); idx >= 0 {
						name = strings.TrimSpace(rest[:idx])
						break
					}
				}
				if name == "" {
					name = strings.TrimSpace(rest)
				}
				// For Erlang: -module(name) → name is inside parens
				if language == "erlang" && strings.HasSuffix(pat, "(") {
					name = strings.TrimSuffix(name, ")")
				}
				if name != "" {
					if len(name) > 80 {
						name = name[:80]
					}
					syms = append(syms, db.Symbol{
						Name:      name,
						Kind:      "section",
						FilePath:  filePath,
						StartLine: lineNum,
						EndLine:   lineNum,
						Signature: trimmed,
						Language:  language,
					})
					matched = true
					break
				}
			}
		}
		_ = matched
	}
	return syms
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/parser/walk_fallback.go
git commit -m "feat(parser): add line-based fallback walker for 15+ languages"
```

---

## Task 8: Update parser.go (wire all new parsers)

**Files:**
- Modify: `internal/parser/parser.go`

- [ ] **Step 1: Add new imports to `parser.go`**

Replace the import block at the top of `internal/parser/parser.go` with:

```go
import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/css"
	"github.com/smacker/go-tree-sitter/elixir"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/groovy"
	"github.com/smacker/go-tree-sitter/hcl"
	"github.com/smacker/go-tree-sitter/html"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/kotlin"
	"github.com/smacker/go-tree-sitter/lua"
	tree_sitter_markdown "github.com/smacker/go-tree-sitter/markdown/tree-sitter-markdown"
	"github.com/smacker/go-tree-sitter/ocaml"
	"github.com/smacker/go-tree-sitter/php"
	"github.com/smacker/go-tree-sitter/protobuf"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/ruby"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/scala"
	"github.com/smacker/go-tree-sitter/sql"
	"github.com/smacker/go-tree-sitter/svelte"
	"github.com/smacker/go-tree-sitter/swift"
	"github.com/smacker/go-tree-sitter/toml"
	tstyp "github.com/smacker/go-tree-sitter/typescript/typescript"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/yaml"

	"gocode-outline-graph/internal/db"
)
```

- [ ] **Step 2: Replace the `New()` function in `parser.go`**

Replace the existing `New()` function (lines 30-51) with:

```go
func New() *SymbolParser {
	langs := map[string]*sitter.Language{
		// Already existing
		"python":     python.GetLanguage(),
		"javascript": javascript.GetLanguage(),
		"typescript": tstyp.GetLanguage(),
		"tsx":        tsx.GetLanguage(),
		"go":         golang.GetLanguage(),
		"rust":       rust.GetLanguage(),
		"java":       java.GetLanguage(),
		"c":          c.GetLanguage(),
		"cpp":        cpp.GetLanguage(),
		"ruby":       ruby.GetLanguage(),
		"bash":       bash.GetLanguage(),
		// Systems
		"csharp": csharp.GetLanguage(),
		"php":    php.GetLanguage(),
		"swift":  swift.GetLanguage(),
		"kotlin": kotlin.GetLanguage(),
		"scala":  scala.GetLanguage(),
		// Config
		"yaml":  yaml.GetLanguage(),
		"toml":  toml.GetLanguage(),
		"hcl":   hcl.GetLanguage(),
		"proto": protobuf.GetLanguage(),
		// Web
		"html":   html.GetLanguage(),
		"css":    css.GetLanguage(),
		"svelte": svelte.GetLanguage(),
		// Scripting
		"lua":    lua.GetLanguage(),
		"elixir": elixir.GetLanguage(),
		"groovy": groovy.GetLanguage(),
		"ocaml":  ocaml.GetLanguage(),
		// Markup
		"markdown":   tree_sitter_markdown.GetLanguage(),
		"sql":        sql.GetLanguage(),
		"dockerfile": bash.GetLanguage(), // Dockerfile grammar falls back to bash for now
	}
	sp := &SymbolParser{parsers: make(map[string]*sitter.Parser, len(langs))}
	for name, lang := range langs {
		p := sitter.NewParser()
		p.SetLanguage(lang)
		sp.parsers[name] = p
	}
	return sp
}
```

Add `"github.com/smacker/go-tree-sitter/dockerfile"` to the imports (alongside the other grammar imports) and use `dockerfile.GetLanguage()` — it has a standard `GetLanguage() *sitter.Language` function.

- [ ] **Step 4: Replace the `Parse()` function's switch statement in `parser.go`**

Replace the switch inside `Parse()` (starting at line 67) with:

```go
	switch language {
	// Existing
	case "python":
		return walkPython(root, src, filePath, "")
	case "javascript":
		return walkJS(root, src, filePath, "", false)
	case "typescript", "tsx":
		return walkJS(root, src, filePath, "", true)
	case "go":
		return walkGo(root, src, filePath, "")
	case "rust":
		return walkRust(root, src, filePath, "")
	case "java":
		return walkJava(root, src, filePath, "")
	case "c":
		return walkC(root, src, filePath, "", false)
	case "cpp":
		return walkC(root, src, filePath, "", true)
	case "ruby":
		return walkRuby(root, src, filePath, "")
	case "bash":
		return walkBash(root, src, filePath)
	// Systems
	case "csharp":
		return walkCSharp(root, src, filePath, "")
	case "php":
		return walkPHP(root, src, filePath, "")
	case "swift":
		return walkSwift(root, src, filePath, "")
	case "kotlin":
		return walkKotlin(root, src, filePath, "")
	case "scala":
		return walkScala(root, src, filePath, "")
	// Config
	case "yaml":
		return walkYAML(root, src, filePath)
	case "toml":
		return walkTOML(root, src, filePath)
	case "hcl":
		return walkHCL(root, src, filePath)
	case "proto":
		return walkProto(root, src, filePath, "")
	case "json":
		return walkJSONBytes(src, filePath)
	// Web
	case "html":
		return walkHTML(root, src, filePath)
	case "css":
		return walkCSS(root, src, filePath)
	case "svelte":
		return walkSvelte(root, src, filePath)
	// Scripting
	case "lua":
		return walkLua(root, src, filePath, "")
	case "elixir":
		return walkElixir(root, src, filePath, "")
	case "groovy":
		return walkGroovy(root, src, filePath, "")
	case "ocaml":
		return walkOCaml(root, src, filePath, "")
	// Markup
	case "markdown":
		return walkMarkdown(root, src, filePath)
	case "sql":
		return walkSQL(root, src, filePath)
	case "dockerfile":
		return walkDockerfile(root, src, filePath)
	}
	return nil
```

Note: `json` calls `walkJSONBytes(src, filePath)` directly without using the tree-sitter `root` node since it uses `encoding/json` instead. The `Parse()` function needs a special case for json before the tree-sitter parse step. Update `Parse()` to handle json before calling `p.ParseCtx`:

```go
func (sp *SymbolParser) Parse(filePath string, src []byte, language string) []db.Symbol {
	// JSON uses stdlib, not tree-sitter.
	if language == "json" {
		return walkJSONBytes(src, filePath)
	}

	// Fallback languages use line-based extraction.
	switch language {
	case "dart", "zig", "clojure", "erlang", "haskell", "nix", "fish",
		"perl", "r", "powershell", "batch", "graphql", "xml", "make":
		return walkLineBased(src, filePath, language)
	}

	p, ok := sp.parsers[language]
	if !ok {
		return nil
	}
	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return nil
	}
	root := tree.RootNode()
	if root == nil {
		return nil
	}
	switch language {
	// ... (full switch as above)
	}
	return nil
}
```

- [ ] **Step 5: Build to check for compile errors**

```bash
cd /home/rushikesh.sakharle/Projects/gocode-outline-graph
go build -buildvcs=false -tags fts5 ./... 2>&1
```

Expected: compiles cleanly. Fix any import or signature mismatches shown.

- [ ] **Step 6: Run languages test**

```bash
go test -tags fts5 ./internal/parser/... -v 2>&1
```

Expected: `TestIsSupported_acceptsImplementedLanguages` PASS, `TestIsSupported_rejectsUnknownExtensions` PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/parser/parser.go
git commit -m "feat(parser): wire all new language walkers into Parse() dispatch"
```

---

## Task 9: Add resolve_edit_target MCP Tool

**Files:**
- Modify: `internal/server/tools.go`

- [ ] **Step 1: Add tool definition to `allTools()` in `internal/server/tools.go`**

In `allTools()`, after the `find_by_kind` entry (line ~221), add:

```go
		{
			Name:        "resolve_edit_target",
			Description: "Find the best edit targets for a natural-language query. Returns top candidates ranked by hybrid FTS+keyword search with Reciprocal Rank Fusion (RRF).",
			InputSchema: ToolSchema{
				Type: "object",
				Properties: map[string]PropSchema{
					"query":        {Type: "string", Description: "Natural language description of what you want to edit"},
					"project_path": {Type: "string", Description: "Project root (used to locate the index)"},
					"limit":        {Type: "integer", Description: "Number of candidates to return (default 5)"},
				},
				Required: []string{"query"},
			},
		},
```

- [ ] **Step 2: Add the handler function to `tools.go`**

Add this function after the existing `handleFindByKind` function (near the end of tools.go):

```go
// 15. resolve_edit_target
func handleResolveEditTarget(raw json.RawMessage) interface{} {
	var args struct {
		Query       string `json:"query"`
		ProjectPath string `json:"project_path"`
		Limit       int    `json:"limit"`
	}
	if raw != nil {
		json.Unmarshal(raw, &args) //nolint:errcheck
	}

	if args.Query == "" {
		return toolError("query is required")
	}
	if args.ProjectPath == "" {
		args.ProjectPath = "."
	}
	if args.Limit <= 0 {
		args.Limit = 5
	}

	_, _, srch, err := GetComponents(args.ProjectPath)
	if err != nil {
		return toolError("init components: %v", err)
	}

	results, err := srch.ResolveEditTarget(args.Query, args.Limit)
	if err != nil {
		return toolError("resolve edit target: %v", err)
	}

	if len(results) == 0 {
		return textResult(fmt.Sprintf("No edit targets found for %q", args.Query))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Top %d edit targets for %q:\n\n", len(results), args.Query)
	for i, s := range results {
		fmt.Fprintf(&sb, "%d. %s %s\n   %s:%d-%d\n", i+1, s.Kind, s.Name, s.FilePath, s.StartLine, s.EndLine)
		if s.Signature != "" {
			fmt.Fprintf(&sb, "   %s\n", s.Signature)
		}
	}
	return textResult(sb.String())
}
```

- [ ] **Step 3: Add dispatch case in `callTool()` switch**

In the `callTool()` switch statement (around line 274), add before `default:`:

```go
	case "resolve_edit_target":
		return handleResolveEditTarget(p.Arguments)
```

- [ ] **Step 4: Build**

```bash
go build -buildvcs=false -tags fts5 ./... 2>&1
```

Expected: clean build.

- [ ] **Step 5: Commit**

```bash
git add internal/server/tools.go
git commit -m "feat(server): expose resolve_edit_target as MCP tool"
```

---

## Task 10: Add CLI Commands (doctor, export, callers, callees)

**Files:**
- Modify: `cmd/code-outline-graph/main.go`

- [ ] **Step 1: Add `cmdDoctor` function to `main.go`**

Add after `cmdPrune` (around line 490):

```go
func cmdDoctor(args []string) {
	positional, _ := parseFlags(args)
	rawPath := "."
	if len(positional) > 0 {
		rawPath = positional[0]
	}

	projectPath, err := paths.ResolveProjectPath(rawPath)
	if err != nil {
		errorf("cannot resolve path: %v", err)
	}

	pass := func(label string) {
		fmt.Printf("  %s✓%s %s\n", colorGreen, colorReset, label)
	}
	fail := func(label string, reason string) {
		fmt.Printf("  %s✗%s %s: %s\n", colorRed, colorReset, label, reason)
	}

	fmt.Printf("%sDoctor: %s%s\n", colorBold, projectPath, colorReset)
	allOK := true

	// 1. DB file exists
	dbPath := paths.ProjectDBPath(projectPath)
	if _, statErr := os.Stat(dbPath); statErr != nil {
		fail("DB file", "not found at "+dbPath)
		allOK = false
	} else {
		pass("DB file exists: " + dbPath)
	}

	// 2. DB integrity check
	database, dbErr := db.Open(dbPath)
	if dbErr != nil {
		fail("DB open", dbErr.Error())
		allOK = false
	} else {
		defer database.Close()
		fileCount, symbolCount, statsErr := database.GetFileStats()
		if statsErr != nil {
			fail("DB integrity", statsErr.Error())
			allOK = false
		} else {
			pass(fmt.Sprintf("DB integrity OK (%d files, %d symbols)", fileCount, symbolCount))
		}
	}

	// 3. Parser initializes
	func() {
		defer func() {
			if r := recover(); r != nil {
				fail("Parser init", fmt.Sprintf("panic: %v", r))
				allOK = false
			}
		}()
		p := indexer.NewParser()
		if p != nil {
			pass("Parser initialized for all languages")
		}
	}()

	// 4. MCP config files
	mcpJSON := filepath.Join(projectPath, ".mcp.json")
	if _, statErr := os.Stat(mcpJSON); statErr == nil {
		pass("MCP config: .mcp.json found")
	} else {
		fail("MCP config", ".mcp.json not found (run: code-outline-graph-go install .)")
		allOK = false
	}

	if !allOK {
		fmt.Printf("\n%s✗ Some checks failed.%s\n", colorRed, colorReset)
		os.Exit(1)
	}
	fmt.Printf("\n%s✓ All checks passed.%s\n", colorGreen, colorReset)
}
```

Note: `cmdDoctor` calls `indexer.NewParser()`. The indexer package doesn't currently export a parser constructor. Add a thin export to `internal/indexer/indexer.go`:

```go
// NewParser creates a SymbolParser for health-check purposes.
func NewParser() *parser.SymbolParser {
	return parser.New()
}
```

Add that line to `internal/indexer/indexer.go` after the existing imports section. Check that indexer.go already imports `gocode-outline-graph/internal/parser` — if so just add the function. If not, add the import too.

- [ ] **Step 2: Add `cmdExport` function to `main.go`**

```go
func cmdExport(args []string) {
	positional, _ := parseFlags(args)

	format := "json"
	output := ""
	var remaining []string
	for i := 0; i < len(positional); i++ {
		switch {
		case positional[i] == "--format" && i+1 < len(positional):
			i++
			format = positional[i]
		case strings.HasPrefix(positional[i], "--format="):
			format = strings.TrimPrefix(positional[i], "--format=")
		case positional[i] == "--output" && i+1 < len(positional):
			i++
			output = positional[i]
		case strings.HasPrefix(positional[i], "--output="):
			output = strings.TrimPrefix(positional[i], "--output=")
		default:
			remaining = append(remaining, positional[i])
		}
	}

	if len(remaining) < 1 {
		fmt.Fprintln(os.Stderr, "usage: code-outline-graph-go export <path> [--format json|csv] [--output file]")
		os.Exit(1)
	}
	rawPath := remaining[0]

	_, database := openProjectDB(rawPath)
	defer database.Close()

	// Fetch all symbols via an empty keyword search (returns everything up to a large limit).
	// For a full export we call a dedicated method.
	syms, err := database.GetAllSymbols()
	if err != nil {
		errorf("export error: %v", err)
	}

	var w *os.File
	if output == "" {
		w = os.Stdout
	} else {
		f, ferr := os.Create(output)
		if ferr != nil {
			errorf("cannot create output file: %v", ferr)
		}
		defer f.Close()
		w = f
	}

	switch format {
	case "csv":
		fmt.Fprintln(w, "id,name,kind,file_path,start_line,end_line,signature,language,parent")
		for _, s := range syms {
			fmt.Fprintf(w, "%d,%q,%q,%q,%d,%d,%q,%q,%q\n",
				s.ID, s.Name, s.Kind, s.FilePath, s.StartLine, s.EndLine, s.Signature, s.Language, s.Parent)
		}
	default: // json
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(syms); err != nil {
			errorf("JSON encode error: %v", err)
		}
	}

	if output != "" {
		stderrf("%s✓ Exported %d symbols to %s%s\n", colorGreen, len(syms), output, colorReset)
	}
}
```

Note: `cmdExport` calls `database.GetAllSymbols()` which doesn't exist yet. Add it to `internal/db/db.go`:

```go
// GetAllSymbols returns all symbols in the database ordered by file_path, start_line.
func (d *Database) GetAllSymbols() ([]Symbol, error) {
	rows, err := d.db.Query(`
		SELECT id, name, kind, file_path, start_line, end_line,
		       start_byte, end_byte, signature, docstring, parent, language
		FROM symbols
		ORDER BY file_path, start_line`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var syms []Symbol
	for rows.Next() {
		var s Symbol
		if err := rows.Scan(&s.ID, &s.Name, &s.Kind, &s.FilePath,
			&s.StartLine, &s.EndLine, &s.StartByte, &s.EndByte,
			&s.Signature, &s.Docstring, &s.Parent, &s.Language); err != nil {
			return nil, err
		}
		syms = append(syms, s)
	}
	return syms, rows.Err()
}
```

- [ ] **Step 3: Add `cmdCallers` and `cmdCallees` functions to `main.go`**

```go
func cmdCallers(args []string) {
	positional, _ := parseFlags(args)
	if len(positional) < 2 {
		fmt.Fprintln(os.Stderr, "usage: code-outline-graph-go callers <path> <function-name>")
		os.Exit(1)
	}
	rawPath := positional[0]
	name := positional[1]

	_, database := openProjectDB(rawPath)
	defer database.Close()

	callers, err := database.GetCallersByName(name)
	if err != nil {
		errorf("callers error: %v", err)
	}

	if len(callers) == 0 {
		fmt.Printf("%sNo callers found for %q%s\n", colorDim, name, colorReset)
		return
	}
	fmt.Printf("%sCallers of %s%s%s:%s\n", colorBold, colorYellow, name, colorBold, colorReset)
	for _, s := range callers {
		fmt.Printf("  %s%s%s  %s%s:%d%s\n",
			colorBold, s.Name, colorReset,
			colorCyan, s.FilePath, s.StartLine, colorReset)
		if s.Signature != "" {
			fmt.Printf("    %s%s%s\n", colorDim, s.Signature, colorReset)
		}
	}
}

func cmdCallees(args []string) {
	positional, _ := parseFlags(args)
	if len(positional) < 2 {
		fmt.Fprintln(os.Stderr, "usage: code-outline-graph-go callees <path> <symbol-name>")
		os.Exit(1)
	}
	rawPath := positional[0]
	name := positional[1]

	_, database := openProjectDB(rawPath)
	defer database.Close()

	// Resolve symbol ID first.
	syms, err := database.GetSymbolsByName(name, "")
	if err != nil || len(syms) == 0 {
		fmt.Printf("%sSymbol %q not found in index%s\n", colorDim, name, colorReset)
		return
	}
	s := syms[0]

	resolved, unresolved, err := database.GetCalleeSymbols(s.ID)
	if err != nil {
		errorf("callees error: %v", err)
	}

	if len(resolved) == 0 && len(unresolved) == 0 {
		fmt.Printf("%sNo callees found for %q%s\n", colorDim, name, colorReset)
		return
	}
	fmt.Printf("%sCallees of %s%s%s:%s\n", colorBold, colorYellow, name, colorBold, colorReset)
	for _, cs := range resolved {
		fmt.Printf("  %s%s%s  %s%s:%d%s\n",
			colorBold, cs.Name, colorReset,
			colorCyan, cs.FilePath, cs.StartLine, colorReset)
	}
	if len(unresolved) > 0 {
		fmt.Printf("\n%sUnresolved (external/stdlib):%s\n", colorDim, colorReset)
		for _, u := range unresolved {
			fmt.Printf("  %s%s%s\n", colorDim, u, colorReset)
		}
	}
}
```

Note: `GetCalleeSymbols` returns `([]Symbol, []string, error)` based on `internal/db/db.go:455`. Check the exact signature and adjust accordingly.

- [ ] **Step 4: Wire new commands into `main()` switch**

In `main()`, add new cases:

```go
	case "doctor":
		cmdDoctor(os.Args[2:])
	case "export":
		cmdExport(os.Args[2:])
	case "callers":
		cmdCallers(os.Args[2:])
	case "callees":
		cmdCallees(os.Args[2:])
```

Also update `printUsage()` to mention the new commands. Find `printUsage()` in main.go and add the 4 new commands to the usage text.

- [ ] **Step 5: Build**

```bash
go build -buildvcs=false -tags fts5 -o code-outline-graph-go ./cmd/code-outline-graph/
```

Expected: binary produced. Fix any compile errors.

- [ ] **Step 6: Quick smoke test**

```bash
./code-outline-graph-go version
./code-outline-graph-go doctor . 2>&1 | head -20
```

- [ ] **Step 7: Commit**

```bash
git add cmd/code-outline-graph/main.go internal/db/db.go internal/indexer/indexer.go
git commit -m "feat(cli): add doctor, export, callers, callees commands"
```

---

## Task 11: Full Build, Test, and Index Smoke Test

**Files:** None (verification only)

- [ ] **Step 1: Run all tests**

```bash
cd /home/rushikesh.sakharle/Projects/gocode-outline-graph
go test -buildvcs=false -tags fts5 ./... 2>&1
```

Expected: all tests pass.

- [ ] **Step 2: Build final binary**

```bash
go build -buildvcs=false -tags fts5 -o code-outline-graph-go ./cmd/code-outline-graph/
```

- [ ] **Step 3: Index the Go project itself and check new languages detected**

```bash
./code-outline-graph-go build .
./code-outline-graph-go status .
```

- [ ] **Step 4: Index the Python project and check multi-language support**

```bash
./code-outline-graph-go build ../code-outline-graph/
./code-outline-graph-go status ../code-outline-graph/
./code-outline-graph-go search ../code-outline-graph/ "class"
```

- [ ] **Step 5: Smoke test resolve_edit_target MCP tool**

Start MCP server and send a test request:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"resolve_edit_target","arguments":{"query":"parse symbols","project_path":".","limit":3}}}' | ./code-outline-graph-go serve 2>/dev/null
```

Expected: JSON response with `content[0].text` containing symbol names.

- [ ] **Step 6: Smoke test doctor command**

```bash
./code-outline-graph-go doctor .
```

Expected: all checks pass with green checkmarks.

- [ ] **Step 7: Smoke test export command**

```bash
./code-outline-graph-go export . --format csv | head -5
./code-outline-graph-go export . --format json | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'{len(d)} symbols exported')"
```

- [ ] **Step 8: Update skill content in main.go**

Find `skillContent` in `cmd/code-outline-graph/main.go` (around line 347). Update the "Supported Languages" section at the bottom to list all 40+ languages now supported.

- [ ] **Step 9: Final commit**

```bash
git add -A
git commit -m "feat: complete Python parity — 40+ languages, doctor/export/callers/callees CLI, resolve_edit_target MCP tool"
```

---

## Dependency Note

All tree-sitter grammar packages are already in `go.mod` under `github.com/smacker/go-tree-sitter v0.0.0-20240827094217-dd81d9e9be82`. No new `go get` commands needed. The new import paths (e.g. `github.com/smacker/go-tree-sitter/csharp`) are subpackages within the same module.

## Common Failure Modes

- **"undefined: walkXxx"** — forgot to add the case to `Parse()` switch or created the file but forgot to add the function
- **Field name mismatch** — tree-sitter node field names vary by grammar. If `field(node, "name", src)` returns empty for a node type, use `nnamed()` + `named()` iteration to find the identifier child
- **Import cycle** — `walk_markup.go` uses `fmt.Sprintf` so needs `"fmt"` in imports
- **json walk returns nil** — ensure the JSON file is a valid object (not array); `walkJSONBytes` returns nil for arrays
- **Dockerfile grammar node types** — if `from_instruction` doesn't match, use `node.Type() == "instruction"` and check the first keyword token
