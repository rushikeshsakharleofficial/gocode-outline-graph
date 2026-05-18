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
		syms = append(syms, sym(node, src, name, "class", filePath, parent, "csharp", "class "+name, "", nil))
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
		if name := field(node, "name", src); name != "" {
			syms = append(syms, sym(node, src, name, "struct", filePath, parent, "csharp", "struct "+name, "", nil))
		}
	case "enum_declaration":
		if name := field(node, "name", src); name != "" {
			syms = append(syms, sym(node, src, name, "enum", filePath, parent, "csharp", "enum "+name, "", nil))
		}
	case "method_declaration", "constructor_declaration":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		params := field(node, "parameters", src)
		ret := field(node, "type", src)
		sig := ret
		if sig != "" {
			sig += " "
		}
		sig += name + params
		calls := collectCalls(node.ChildByFieldName("body"), src)
		syms = append(syms, sym(node, src, name, "method", filePath, parent, "csharp", sig, "", calls))
	case "property_declaration":
		if name := field(node, "name", src); name != "" {
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
		if name := field(node, "name", src); name != "" {
			syms = append(syms, sym(node, src, name, "interface", filePath, parent, "php", "interface "+name, "", nil))
			if body := node.ChildByFieldName("body"); body != nil {
				for i := 0; i < nchildren(body); i++ {
					syms = append(syms, walkPHP(child(body, i), src, filePath, name)...)
				}
			}
		}
	case "trait_declaration":
		if name := field(node, "name", src); name != "" {
			syms = append(syms, sym(node, src, name, "interface", filePath, parent, "php", "trait "+name, "", nil))
		}
	case "enum_declaration":
		if name := field(node, "name", src); name != "" {
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
		syms = append(syms, sym(node, src, name, "method", filePath, parent, "php", name+params, "", calls))
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
			// Swift uses type_identifier for class names
			for i := 0; i < nnamed(node); i++ {
				ch := named(node, i)
				if ch.Type() == "type_identifier" {
					name = text(ch, src)
					break
				}
			}
		}
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
			for i := 0; i < nnamed(node); i++ {
				ch := named(node, i)
				if ch.Type() == "type_identifier" {
					name = text(ch, src)
					break
				}
			}
		}
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
		if name == "" {
			for i := 0; i < nnamed(node); i++ {
				ch := named(node, i)
				if ch.Type() == "type_identifier" {
					name = text(ch, src)
					break
				}
			}
		}
		if name != "" {
			syms = append(syms, sym(node, src, name, "interface", filePath, parent, "swift", "protocol "+name, "", nil))
		}
	case "enum_declaration":
		name := field(node, "name", src)
		if name == "" {
			for i := 0; i < nnamed(node); i++ {
				ch := named(node, i)
				if ch.Type() == "type_identifier" {
					name = text(ch, src)
					break
				}
			}
		}
		if name != "" {
			syms = append(syms, sym(node, src, name, "enum", filePath, parent, "swift", "enum "+name, "", nil))
		}
	case "extension_declaration":
		name := ""
		for i := 0; i < nnamed(node); i++ {
			ch := named(node, i)
			if ch.Type() == "type_identifier" || ch.Type() == "user_type" {
				name = text(ch, src)
				break
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
			for i := 0; i < nnamed(node); i++ {
				ch := named(node, i)
				if ch.Type() == "simple_identifier" {
					name = text(ch, src)
					break
				}
			}
		}
		if name == "" {
			break
		}
		sig := "func " + name
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
		if name := kotlinSimpleName(node, src); name != "" {
			syms = append(syms, sym(node, src, name, "class", filePath, parent, "kotlin", "object "+name, "", nil))
		}
	case "interface_declaration":
		if name := kotlinSimpleName(node, src); name != "" {
			syms = append(syms, sym(node, src, name, "interface", filePath, parent, "kotlin", "interface "+name, "", nil))
		}
	case "function_declaration":
		name := kotlinSimpleName(node, src)
		if name == "" {
			break
		}
		sig := "fun " + name
		calls := collectCalls(node.ChildByFieldName("function_body"), src)
		kind := "function"
		if parent != "" {
			kind = "method"
		}
		syms = append(syms, sym(node, src, name, kind, filePath, parent, "kotlin", sig, "", calls))
	case "property_declaration":
		if name := kotlinSimpleName(node, src); name != "" {
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
		if name := field(node, "name", src); name != "" {
			syms = append(syms, sym(node, src, name, "class", filePath, parent, "scala", "object "+name, "", nil))
		}
	case "trait_definition":
		if name := field(node, "name", src); name != "" {
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
		if name := field(node, "pattern", src); name != "" {
			syms = append(syms, sym(node, src, name, "constant", filePath, parent, "scala", "val "+name, "", nil))
		}
	default:
		for i := 0; i < nchildren(node); i++ {
			syms = append(syms, walkScala(child(node, i), src, filePath, parent)...)
		}
	}
	return syms
}
