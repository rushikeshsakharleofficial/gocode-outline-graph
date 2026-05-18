package parser

import (
	"fmt"
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
			name := ""
			for i := 0; i < nchildren(n); i++ {
				ch := child(n, i)
				if ch.Type() == "atx_heading_content" || ch.Type() == "inline" {
					name = strings.TrimSpace(text(ch, src))
					break
				}
			}
			if name == "" {
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
		switch n.Type() {
		case "create_table_statement":
			if name := sqlObjectName(n, src); name != "" {
				syms = append(syms, sym(n, src, name, "table", filePath, "", "sql", "CREATE TABLE "+name, "", nil))
			}
		case "create_view_statement":
			if name := sqlObjectName(n, src); name != "" {
				syms = append(syms, sym(n, src, name, "view", filePath, "", "sql", "CREATE VIEW "+name, "", nil))
			}
		case "create_function_statement":
			if name := sqlObjectName(n, src); name != "" {
				syms = append(syms, sym(n, src, name, "function", filePath, "", "sql", "CREATE FUNCTION "+name, "", nil))
			}
		case "create_procedure_statement":
			if name := sqlObjectName(n, src); name != "" {
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
		if ch.Type() != "from_instruction" {
			continue
		}
		stageNum++
		name := ""
		// Look for AS alias (image_alias node)
		for j := 0; j < nnamed(ch); j++ {
			nc := named(ch, j)
			if nc.Type() == "image_alias" {
				name = text(nc, src)
				break
			}
		}
		if name == "" {
			// Use image name without tag
			for j := 0; j < nnamed(ch); j++ {
				nc := named(ch, j)
				if nc.Type() == "image_spec" || nc.Type() == "image_name" {
					v := text(nc, src)
					v = strings.Split(v, ":")[0]
					v = strings.Split(v, "@")[0]
					name = strings.TrimSpace(v)
					break
				}
			}
		}
		if name == "" {
			name = fmt.Sprintf("stage%d", stageNum)
		}
		sig := strings.TrimSpace(firstLine(ch, src))
		syms = append(syms, sym(ch, src, name, "section", filePath, "", "dockerfile", sig, "", nil))
	}
	return syms
}
