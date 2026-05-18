// Package parser provides tree-sitter-based multi-language symbol extraction.
package parser

import (
	"path/filepath"
	"strings"
)

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
