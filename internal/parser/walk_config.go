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
	// Walk top-level block_mapping_pair nodes at depth 1.
	var walk func(*sitter.Node, int)
	walk = func(n *sitter.Node, depth int) {
		if n == nil {
			return
		}
		if n.Type() == "block_mapping_pair" && depth <= 2 {
			keyNode := n.ChildByFieldName("key")
			if keyNode != nil {
				name := strings.TrimSpace(text(keyNode, src))
				if name != "" {
					syms = append(syms, sym(n, src, name, "section", filePath, "", "yaml", name+":", "", nil))
				}
			}
			return
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
			name := strings.Trim(strings.TrimSpace(text(ch, src)), "[]")
			if name != "" {
				syms = append(syms, sym(ch, src, name, "section", filePath, "", "toml", "["+name+"]", "", nil))
			}
		case "table_array_element":
			name := strings.Trim(strings.TrimSpace(text(ch, src)), "[]")
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
		name := protoName(node, src)
		if name != "" {
			syms = append(syms, sym(node, src, name, "class", filePath, parent, "proto", "message "+name, "", nil))
			if body := node.ChildByFieldName("body"); body != nil {
				for i := 0; i < nchildren(body); i++ {
					syms = append(syms, walkProto(child(body, i), src, filePath, name)...)
				}
			}
		}
	case "enum_definition":
		if name := protoName(node, src); name != "" {
			syms = append(syms, sym(node, src, name, "enum", filePath, parent, "proto", "enum "+name, "", nil))
		}
	case "service_definition":
		name := protoName(node, src)
		if name != "" {
			syms = append(syms, sym(node, src, name, "interface", filePath, parent, "proto", "service "+name, "", nil))
			if body := node.ChildByFieldName("body"); body != nil {
				for i := 0; i < nchildren(body); i++ {
					syms = append(syms, walkProto(child(body, i), src, filePath, name)...)
				}
			}
		}
	case "rpc_definition":
		if name := protoName(node, src); name != "" {
			syms = append(syms, sym(node, src, name, "function", filePath, parent, "proto", "rpc "+name, "", nil))
		}
	default:
		for i := 0; i < nchildren(node); i++ {
			syms = append(syms, walkProto(child(node, i), src, filePath, parent)...)
		}
	}
	return syms
}

func protoName(node *sitter.Node, src []byte) string {
	for i := 0; i < nnamed(node); i++ {
		ch := named(node, i)
		switch ch.Type() {
		case "message_name", "enum_name", "service_name", "rpc_name", "identifier":
			return text(ch, src)
		}
	}
	return ""
}

// ── JSON (stdlib, no tree-sitter) ────────────────────────────────────────────

// walkJSONBytes extracts top-level keys from a JSON file as section symbols.
// Uses encoding/json since smacker/go-tree-sitter has no JSON grammar.
func walkJSONBytes(src []byte, filePath string) []db.Symbol {
	var top interface{}
	if err := json.Unmarshal(src, &top); err != nil {
		return nil
	}
	obj, ok := top.(map[string]interface{})
	if !ok {
		return nil
	}

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
