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
			tag := htmlTagName(n, src)
			switch tag {
			case "h1", "h2", "h3", "h4", "h5", "h6":
				inner := strings.TrimSpace(htmlInnerText(n, src))
				if len(inner) > 80 {
					inner = inner[:80]
				}
				if inner == "" {
					inner = tag
				}
				syms = append(syms, sym(n, src, inner, "section", filePath, "", "html", "<"+tag+">"+inner+"</"+tag+">", "", nil))
				return
			}
		}
		for i := 0; i < nchildren(n); i++ {
			walk(child(n, i))
		}
	}
	walk(node)
	return syms
}

func htmlTagName(elem *sitter.Node, src []byte) string {
	for i := 0; i < nchildren(elem); i++ {
		ch := child(elem, i)
		if ch.Type() == "start_tag" || ch.Type() == "self_closing_tag" {
			for j := 0; j < nchildren(ch); j++ {
				nc := child(ch, j)
				if nc.Type() == "tag_name" {
					return strings.ToLower(text(nc, src))
				}
			}
		}
	}
	return ""
}

func htmlInnerText(node *sitter.Node, src []byte) string {
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
			// selector { ... } — find selectors child or first non-brace child
			sel := ""
			for i := 0; i < nchildren(n); i++ {
				ch := child(n, i)
				if ch.Type() != "{" && ch.Type() != "block" {
					sel = strings.TrimSpace(text(ch, src))
					break
				}
			}
			if len(sel) > 80 {
				sel = sel[:80]
			}
			if sel != "" {
				syms = append(syms, sym(n, src, sel, "section", filePath, "", "css", sel+" { }", "", nil))
			}
		case "media_statement", "keyframes_statement":
			name := firstLine(n, src)
			if name != "" {
				syms = append(syms, sym(n, src, name, "section", filePath, "", "css", name, "", nil))
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
