// Package parser provides tree-sitter-based multi-language symbol extraction.
package parser

import (
	"path/filepath"
	"strings"
)

// extensionMap maps lowercase file extensions (with leading dot) to tree-sitter language names.
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
	".cs":     "csharp",
	".php":    "php",
	".swift":  "swift",
	".kt":     "kotlin",
	".kts":    "kotlin",
	".scala":  "scala",
	// Config / data
	".yaml":   "yaml",
	".yml":    "yaml",
	".toml":   "toml",
	".tf":     "hcl",
	".hcl":    "hcl",
	".proto":  "proto",
	".json":   "json",
	// Web
	".html":   "html",
	".htm":    "html",
	".css":    "css",
	".scss":   "css",
	".sass":   "css",
	".less":   "css",
	".svelte": "svelte",
	".vue":    "svelte",
	// Scripting / functional
	".lua":    "lua",
	".ex":     "elixir",
	".exs":    "elixir",
	".groovy": "groovy",
	".gradle": "groovy",
	".ml":     "ocaml",
	".mli":    "ocaml",
	// Markup
	".md":    "markdown",
	".mdx":   "markdown",
	".sql":   "sql",
	// Fallback languages (line-based extraction)
	".dart":    "dart",
	".zig":     "zig",
	".clj":     "clojure",
	".cljs":    "clojure",
	".cljc":    "clojure",
	".erl":     "erlang",
	".hrl":     "erlang",
	".hs":      "haskell",
	".lhs":     "haskell",
	".nix":     "nix",
	".fish":    "fish",
	".pl":      "perl",
	".pm":      "perl",
	".t":       "perl",
	".r":       "r",
	".R":       "r",
	".ps1":     "powershell",
	".psm1":    "powershell",
	".bat":     "batch",
	".cmd":     "batch",
	".graphql": "graphql",
	".gql":     "graphql",
	".xml":     "xml",
	".mk":      "make",
}

// basenameMap maps exact filenames (no path, case-sensitive) to language names.
var basenameMap = map[string]string{
	"Dockerfile":      "dockerfile",
	"dockerfile":      "dockerfile",
	"Dockerfile.dev":  "dockerfile",
	"Dockerfile.prod": "dockerfile",
	"Makefile":        "make",
	"makefile":        "make",
	"GNUmakefile":     "make",
}

// DetectLanguage returns the tree-sitter language name for a given file path.
// Returns an empty string for unsupported files.
func DetectLanguage(filePath string) string {
	base := filepath.Base(filePath)

	// Check exact basename match first (e.g. Dockerfile).
	if lang, ok := basenameMap[base]; ok {
		return lang
	}

	// Derive extension; handle files like ".R" (extension == ".R").
	ext := filepath.Ext(base)
	if ext == "" {
		return ""
	}

	// Case-sensitive lookup first (handles ".R" vs ".r").
	if lang, ok := extensionMap[ext]; ok {
		return lang
	}

	// Case-insensitive fallback.
	if lang, ok := extensionMap[strings.ToLower(ext)]; ok {
		return lang
	}

	return ""
}

// IsSupported returns true if the file extension is recognised by DetectLanguage.
func IsSupported(filePath string) bool {
	return DetectLanguage(filePath) != ""
}
