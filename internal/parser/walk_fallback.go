package parser

import (
	"bufio"
	"bytes"
	"strings"

	"gocode-outline-graph/internal/db"
)

// lineBasedPatterns maps language name to keyword prefixes that signal a
// symbol boundary. Each matching line becomes a "section" symbol.
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
	"graphql":    {"type ", "input ", "enum ", "union ", "interface ", "scalar ", "directive "},
	"xml":        {"<"},
	// batch and make handled specially
	"batch": {},
	"make":  {},
}

// walkLineBased produces section symbols by scanning source lines for
// language-specific keyword markers. This is the fallback for languages
// without Go tree-sitter bindings.
func walkLineBased(src []byte, filePath, language string) []db.Symbol {
	switch language {
	case "make":
		return walkMake(src, filePath)
	case "batch":
		return walkBatch(src, filePath)
	}

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

		if trimmed == "" {
			continue
		}
		// Skip common comment prefixes
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") ||
			strings.HasPrefix(trimmed, "--") || strings.HasPrefix(trimmed, ";") {
			continue
		}

		for _, pat := range patterns {
			if strings.HasPrefix(trimmed, pat) {
				rest := strings.TrimSpace(trimmed[len(pat):])
				name := extractFirstToken(rest)
				// Erlang: -module(name). → strip parens/dot
				if language == "erlang" && strings.HasPrefix(pat, "-module(") {
					name = strings.TrimRight(name, ").")
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
					break
				}
			}
		}
	}
	return syms
}

// extractFirstToken returns the first token from s, stopping at (, {, [, space, = or :
func extractFirstToken(s string) string {
	for i, ch := range s {
		switch ch {
		case '(', '{', '[', ' ', '\t', '=', ':', ',', ';':
			return strings.TrimSpace(s[:i])
		}
	}
	return strings.TrimSpace(s)
}

func walkMake(src []byte, filePath string) []db.Symbol {
	var syms []db.Symbol
	scanner := bufio.NewScanner(bytes.NewReader(src))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		// Makefile targets: non-indented, contain ":", not a comment or variable
		if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, " ") ||
			strings.HasPrefix(line, "#") || strings.HasPrefix(line, ".") {
			continue
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			name := strings.TrimSpace(line[:idx])
			// Skip variable assignments (name =) and phony
			if strings.ContainsAny(name, "= \t$") || name == "" {
				continue
			}
			syms = append(syms, db.Symbol{
				Name:      name,
				Kind:      "section",
				FilePath:  filePath,
				StartLine: lineNum,
				EndLine:   lineNum,
				Signature: strings.TrimSpace(line),
				Language:  "make",
			})
		}
	}
	return syms
}

func walkBatch(src []byte, filePath string) []db.Symbol {
	var syms []db.Symbol
	scanner := bufio.NewScanner(bytes.NewReader(src))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		// Batch labels: :labelname (not ::comment)
		if strings.HasPrefix(trimmed, ":") && len(trimmed) > 1 && trimmed[1] != ':' {
			rest := strings.TrimSpace(trimmed[1:])
			if fields := strings.Fields(rest); len(fields) > 0 {
				name := fields[0]
				syms = append(syms, db.Symbol{
					Name:      name,
					Kind:      "section",
					FilePath:  filePath,
					StartLine: lineNum,
					EndLine:   lineNum,
					Signature: trimmed,
					Language:  "batch",
				})
			}
		}
	}
	return syms
}
