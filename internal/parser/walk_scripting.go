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
		name := luaFuncName(node, src)
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
		name := ""
		for i := 0; i < nnamed(node); i++ {
			ch := named(node, i)
			if ch.Type() == "identifier" {
				name = text(ch, src)
				break
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

func luaFuncName(node *sitter.Node, src []byte) string {
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return text(nameNode, src)
	}
	// Lua function name may be a dotted/method expression
	for i := 0; i < nnamed(node); i++ {
		ch := named(node, i)
		switch ch.Type() {
		case "method_index_expression", "dot_index_expression", "identifier":
			return text(ch, src)
		}
	}
	return ""
}

// ── Elixir ───────────────────────────────────────────────────────────────────

func walkElixir(node *sitter.Node, src []byte, filePath, parent string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	t := node.Type()
	if t == "call" {
		// Elixir is call-based: defmodule, def, defp, defmacro look like calls
		funcNode := node.ChildByFieldName("target")
		if funcNode == nil && nnamed(node) > 0 {
			funcNode = named(node, 0)
		}
		if funcNode != nil {
			macroName := text(funcNode, src)
			switch macroName {
			case "defmodule":
				args := node.ChildByFieldName("arguments")
				if args != nil && nnamed(args) > 0 {
					modName := strings.TrimSpace(text(named(args, 0), src))
					if modName != "" {
						syms = append(syms, sym(node, src, modName, "module", filePath, parent, "elixir", "defmodule "+modName, "", nil))
						// Walk do block body
						for i := 0; i < nchildren(node); i++ {
							ch := child(node, i)
							if ch.Type() == "do_block" {
								for j := 0; j < nchildren(ch); j++ {
									syms = append(syms, walkElixir(child(ch, j), src, filePath, modName)...)
								}
								break
							}
						}
					}
				}
			case "def", "defp", "defmacro", "defmacrop":
				args := node.ChildByFieldName("arguments")
				if args != nil && nnamed(args) > 0 {
					firstArg := named(args, 0)
					name := elixirFuncName(firstArg, src)
					if name != "" {
						sig := macroName + " " + firstLine(node, src)
						calls := collectCalls(node, src)
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

func elixirFuncName(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	// def name(args) → node is a call with target=identifier
	if node.Type() == "call" {
		target := node.ChildByFieldName("target")
		if target != nil {
			return text(target, src)
		}
	}
	if node.Type() == "identifier" {
		return text(node, src)
	}
	// Get text and strip anything after (
	raw := text(node, src)
	if idx := strings.Index(raw, "("); idx >= 0 {
		raw = raw[:idx]
	}
	return strings.TrimSpace(raw)
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
		name := ocamlUpperName(node, src)
		if name != "" {
			syms = append(syms, sym(node, src, name, "module", filePath, parent, "ocaml", "module "+name, "", nil))
			if body := node.ChildByFieldName("body"); body != nil {
				for i := 0; i < nchildren(body); i++ {
					syms = append(syms, walkOCaml(child(body, i), src, filePath, name)...)
				}
			}
		}
	case "let_binding":
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
				name := ocamlLowerName(ch, src)
				if name != "" {
					syms = append(syms, sym(ch, src, name, "type", filePath, parent, "ocaml", "type "+name, "", nil))
				}
			}
		}
	case "value_specification":
		name := ocamlLowerName(node, src)
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

func ocamlUpperName(node *sitter.Node, src []byte) string {
	for i := 0; i < nnamed(node); i++ {
		ch := named(node, i)
		if ch.Type() == "module_name" || ch.Type() == "upper_case_identifier" {
			return text(ch, src)
		}
	}
	if n := node.ChildByFieldName("name"); n != nil {
		return text(n, src)
	}
	return ""
}

func ocamlLowerName(node *sitter.Node, src []byte) string {
	for i := 0; i < nnamed(node); i++ {
		ch := named(node, i)
		if ch.Type() == "value_name" || ch.Type() == "lowercase_identifier" || ch.Type() == "type_name" {
			return text(ch, src)
		}
	}
	if n := node.ChildByFieldName("name"); n != nil {
		return text(n, src)
	}
	return ""
}
