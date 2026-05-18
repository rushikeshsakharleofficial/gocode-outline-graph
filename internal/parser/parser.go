package parser

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/css"
	"github.com/smacker/go-tree-sitter/dockerfile"
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

// SymbolParser extracts symbols from source files via tree-sitter.
// NOT goroutine-safe — create one per goroutine.
type SymbolParser struct {
	parsers map[string]*sitter.Parser
}

// New creates a SymbolParser. Each goroutine must call New() independently.
func New() *SymbolParser {
	langs := map[string]*sitter.Language{
		// Original languages
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
		// Markup / infra
		"markdown":   tree_sitter_markdown.GetLanguage(),
		"sql":        sql.GetLanguage(),
		"dockerfile": dockerfile.GetLanguage(),
	}
	sp := &SymbolParser{parsers: make(map[string]*sitter.Parser, len(langs))}
	for name, lang := range langs {
		p := sitter.NewParser()
		p.SetLanguage(lang)
		sp.parsers[name] = p
	}
	return sp
}

// Parse extracts symbols from src for the given language. Returns nil for unsupported languages.
func (sp *SymbolParser) Parse(filePath string, src []byte, language string) []db.Symbol {
	// JSON uses stdlib encoding/json, not tree-sitter.
	if language == "json" {
		return walkJSONBytes(src, filePath)
	}

	// Fallback languages use line-based extraction — no tree-sitter grammar available.
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
	// Original
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
	// Markup / infra
	case "markdown":
		return walkMarkdown(root, src, filePath)
	case "sql":
		return walkSQL(root, src, filePath)
	case "dockerfile":
		return walkDockerfile(root, src, filePath)
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func text(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	s, e := int(node.StartByte()), int(node.EndByte())
	if e > len(src) {
		e = len(src)
	}
	return string(src[s:e])
}

func field(node *sitter.Node, name string, src []byte) string {
	return text(node.ChildByFieldName(name), src)
}

func sline(n *sitter.Node) int { return int(n.StartPoint().Row) + 1 }
func eline(n *sitter.Node) int { return int(n.EndPoint().Row) + 1 }

func nchildren(n *sitter.Node) int   { return int(n.ChildCount()) }
func child(n *sitter.Node, i int) *sitter.Node { return n.Child(i) }
func nnamed(n *sitter.Node) int    { return int(n.NamedChildCount()) }
func named(n *sitter.Node, i int) *sitter.Node { return n.NamedChild(i) }

func sym(node *sitter.Node, src []byte, name, kind, filePath, parent, lang, sig, doc string, calls []string) db.Symbol {
	return db.Symbol{
		Name:      name,
		Kind:      kind,
		FilePath:  filePath,
		StartLine: sline(node),
		EndLine:   eline(node),
		StartByte: int(node.StartByte()),
		EndByte:   int(node.EndByte()),
		Signature: sig,
		Docstring: doc,
		Parent:    parent,
		Language:  lang,
		Calls:     calls,
	}
}

// collectCalls walks a subtree for call_expression nodes.
func collectCalls(node *sitter.Node, src []byte) []string {
	if node == nil {
		return nil
	}
	seen := map[string]bool{}
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		t := n.Type()
		if t == "call_expression" || t == "call" || t == "function_call" || t == "method_invocation" {
			fn := n.ChildByFieldName("function")
			if fn == nil && nchildren(n) > 0 {
				fn = child(n, 0)
			}
			if fn != nil {
				name := ""
				switch fn.Type() {
				case "identifier":
					name = text(fn, src)
				case "attribute", "member_expression", "field_expression", "scoped_identifier":
					for i := nchildren(fn) - 1; i >= 0; i-- {
						ch := child(fn, i)
						t := ch.Type()
						if t == "identifier" || t == "field_identifier" || t == "property_identifier" {
							name = text(ch, src)
							break
						}
					}
				}
				if name != "" {
					seen[name] = true
				}
			}
		}
		for i := 0; i < nchildren(n); i++ {
			walk(child(n, i))
		}
	}
	walk(node)
	if len(seen) == 0 {
		return nil
	}
	calls := make([]string, 0, len(seen))
	for k := range seen {
		calls = append(calls, k)
	}
	return calls
}

// ── Python ───────────────────────────────────────────────────────────────────

func walkPython(node *sitter.Node, src []byte, filePath, parent string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	switch node.Type() {
	case "function_definition", "async_function_definition":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		sig := "def " + name + field(node, "parameters", src)
		doc := ""
		body := node.ChildByFieldName("body")
		if body != nil {
			doc = pythonDoc(body, src)
		}
		calls := collectCalls(body, src)
		syms = append(syms, sym(node, src, name, "function", filePath, parent, "python", sig, doc, calls))
		if body != nil {
			syms = append(syms, childrenPy(body, src, filePath, name)...)
		}
	case "class_definition":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		sig := "class " + name
		if bases := node.ChildByFieldName("superclasses"); bases != nil {
			sig += text(bases, src)
		}
		body := node.ChildByFieldName("body")
		doc := ""
		if body != nil {
			doc = pythonDoc(body, src)
		}
		syms = append(syms, sym(node, src, name, "class", filePath, parent, "python", sig, doc, nil))
		if body != nil {
			syms = append(syms, childrenPy(body, src, filePath, name)...)
		}
	case "decorated_definition":
		for i := 0; i < nchildren(node); i++ {
			ch := child(node, i)
			t := ch.Type()
			if t == "function_definition" || t == "async_function_definition" || t == "class_definition" {
				inner := walkPython(ch, src, filePath, parent)
				for j := range inner {
					inner[j].StartLine = sline(node)
					inner[j].StartByte = int(node.StartByte())
				}
				syms = append(syms, inner...)
				break
			}
		}
	default:
		for i := 0; i < nchildren(node); i++ {
			syms = append(syms, walkPython(child(node, i), src, filePath, parent)...)
		}
	}
	return syms
}

func childrenPy(node *sitter.Node, src []byte, filePath, parent string) []db.Symbol {
	var syms []db.Symbol
	for i := 0; i < nchildren(node); i++ {
		syms = append(syms, walkPython(child(node, i), src, filePath, parent)...)
	}
	return syms
}

func pythonDoc(body *sitter.Node, src []byte) string {
	for i := 0; i < nchildren(body); i++ {
		ch := child(body, i)
		if ch.Type() == "expression_statement" {
			for j := 0; j < nchildren(ch); j++ {
				s := child(ch, j)
				if s.Type() == "string" {
					return strings.TrimSpace(strings.Trim(text(s, src), `"'`))
				}
			}
		}
		break
	}
	return ""
}

// ── JavaScript / TypeScript ───────────────────────────────────────────────────

func walkJS(node *sitter.Node, src []byte, filePath, parent string, ts bool) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	t := node.Type()
	switch t {
	case "function_declaration", "function":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		sig := "function " + name + field(node, "parameters", src)
		calls := collectCalls(node.ChildByFieldName("body"), src)
		syms = append(syms, sym(node, src, name, "function", filePath, parent, "javascript", sig, "", calls))
		if body := node.ChildByFieldName("body"); body != nil {
			syms = append(syms, childrenJS(body, src, filePath, parent, ts)...)
		}
	case "class_declaration", "class":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		syms = append(syms, sym(node, src, name, "class", filePath, parent, "javascript", "class "+name, "", nil))
		if body := node.ChildByFieldName("body"); body != nil {
			syms = append(syms, childrenJS(body, src, filePath, name, ts)...)
		}
	case "method_definition":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		sig := name + field(node, "parameters", src)
		calls := collectCalls(node.ChildByFieldName("body"), src)
		syms = append(syms, sym(node, src, name, "method", filePath, parent, "javascript", sig, "", calls))
	case "variable_declarator":
		name := field(node, "name", src)
		val := node.ChildByFieldName("value")
		if name != "" && val != nil && (val.Type() == "arrow_function" || val.Type() == "function") {
			calls := collectCalls(val.ChildByFieldName("body"), src)
			syms = append(syms, sym(node, src, name, "function", filePath, parent, "javascript", name+" = =>", "", calls))
		}
	case "export_statement":
		for i := 0; i < nchildren(node); i++ {
			ch := child(node, i)
			ct := ch.Type()
			if ct == "function_declaration" || ct == "class_declaration" ||
				ct == "variable_declaration" || ct == "lexical_declaration" {
				syms = append(syms, walkJS(ch, src, filePath, parent, ts)...)
			}
		}
	case "lexical_declaration", "variable_declaration":
		syms = append(syms, childrenJS(node, src, filePath, parent, ts)...)
	case "interface_declaration":
		if ts {
			name := field(node, "name", src)
			if name != "" {
				syms = append(syms, sym(node, src, name, "interface", filePath, parent, "typescript", "interface "+name, "", nil))
				if body := node.ChildByFieldName("body"); body != nil {
					syms = append(syms, childrenJS(body, src, filePath, name, ts)...)
				}
			}
		}
	case "type_alias_declaration":
		if ts {
			name := field(node, "name", src)
			if name != "" {
				syms = append(syms, sym(node, src, name, "type", filePath, parent, "typescript", "type "+name, "", nil))
			}
		}
	case "enum_declaration":
		if ts {
			name := field(node, "name", src)
			if name != "" {
				syms = append(syms, sym(node, src, name, "enum", filePath, parent, "typescript", "enum "+name, "", nil))
			}
		}
	default:
		syms = append(syms, childrenJS(node, src, filePath, parent, ts)...)
	}
	return syms
}

func childrenJS(node *sitter.Node, src []byte, filePath, parent string, ts bool) []db.Symbol {
	var syms []db.Symbol
	for i := 0; i < nchildren(node); i++ {
		syms = append(syms, walkJS(child(node, i), src, filePath, parent, ts)...)
	}
	return syms
}

// ── Go ────────────────────────────────────────────────────────────────────────

func walkGo(node *sitter.Node, src []byte, filePath, parent string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	t := node.Type()
	switch t {
	case "function_declaration":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		params := field(node, "parameters", src)
		result := field(node, "result", src)
		sig := "func " + name + params
		if result != "" {
			sig += " " + result
		}
		calls := collectCalls(node.ChildByFieldName("body"), src)
		syms = append(syms, sym(node, src, name, "function", filePath, parent, "go", sig, "", calls))
	case "method_declaration":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		recv := field(node, "receiver", src)
		params := field(node, "parameters", src)
		result := field(node, "result", src)
		sig := "func " + recv + " " + name + params
		if result != "" {
			sig += " " + result
		}
		recvType := goReceiverType(node.ChildByFieldName("receiver"), src)
		calls := collectCalls(node.ChildByFieldName("body"), src)
		syms = append(syms, sym(node, src, name, "method", filePath, recvType, "go", sig, "", calls))
	case "type_declaration":
		for i := 0; i < nchildren(node); i++ {
			ch := child(node, i)
			if ch.Type() != "type_spec" {
				continue
			}
			name := field(ch, "name", src)
			if name == "" {
				continue
			}
			typeVal := ch.ChildByFieldName("type")
			kind := "type"
			if typeVal != nil {
				switch typeVal.Type() {
				case "struct_type":
					kind = "struct"
				case "interface_type":
					kind = "interface"
				}
			}
			sigStr := "type " + name
			if typeVal != nil {
				tv := text(typeVal, src)
				if len(tv) > 80 {
					tv = tv[:80] + "..."
				}
				sigStr += " " + tv
			}
			syms = append(syms, sym(ch, src, name, kind, filePath, parent, "go", sigStr, "", nil))
		}
	case "const_declaration":
		for i := 0; i < nchildren(node); i++ {
			ch := child(node, i)
			if ch.Type() != "const_spec" {
				continue
			}
			name := field(ch, "name", src)
			if name == "" && nnamed(ch) > 0 {
				name = text(named(ch, 0), src)
			}
			if name != "" {
				syms = append(syms, sym(ch, src, name, "constant", filePath, parent, "go", "const "+name, "", nil))
			}
		}
	case "var_declaration":
		for i := 0; i < nchildren(node); i++ {
			ch := child(node, i)
			if ch.Type() != "var_spec" {
				continue
			}
			name := field(ch, "name", src)
			if name == "" && nnamed(ch) > 0 {
				name = text(named(ch, 0), src)
			}
			if name != "" {
				syms = append(syms, sym(ch, src, name, "variable", filePath, parent, "go", "var "+name, "", nil))
			}
		}
	default:
		for i := 0; i < nchildren(node); i++ {
			syms = append(syms, walkGo(child(node, i), src, filePath, parent)...)
		}
	}
	return syms
}

func goReceiverType(recv *sitter.Node, src []byte) string {
	if recv == nil {
		return ""
	}
	var find func(*sitter.Node) string
	find = func(n *sitter.Node) string {
		if n == nil {
			return ""
		}
		if n.Type() == "type_identifier" {
			return text(n, src)
		}
		for i := 0; i < nchildren(n); i++ {
			if s := find(child(n, i)); s != "" {
				return s
			}
		}
		return ""
	}
	return find(recv)
}

// ── Rust ─────────────────────────────────────────────────────────────────────

func walkRust(node *sitter.Node, src []byte, filePath, parent string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	t := node.Type()
	switch t {
	case "function_item":
		name := field(node, "name", src)
		if name == "" {
			break
		}
		params := field(node, "parameters", src)
		ret := field(node, "return_type", src)
		sig := "fn " + name + params
		if ret != "" {
			sig += " -> " + ret
		}
		calls := collectCalls(node.ChildByFieldName("body"), src)
		syms = append(syms, sym(node, src, name, "function", filePath, parent, "rust", sig, "", calls))
	case "struct_item":
		if name := field(node, "name", src); name != "" {
			syms = append(syms, sym(node, src, name, "struct", filePath, parent, "rust", "struct "+name, "", nil))
		}
	case "enum_item":
		if name := field(node, "name", src); name != "" {
			syms = append(syms, sym(node, src, name, "enum", filePath, parent, "rust", "enum "+name, "", nil))
		}
	case "trait_item":
		if name := field(node, "name", src); name != "" {
			syms = append(syms, sym(node, src, name, "interface", filePath, parent, "rust", "trait "+name, "", nil))
			if body := node.ChildByFieldName("body"); body != nil {
				for i := 0; i < nchildren(body); i++ {
					syms = append(syms, walkRust(child(body, i), src, filePath, name)...)
				}
			}
		}
	case "impl_item":
		typeName := field(node, "type", src)
		if typeName != "" {
			sigStr := "impl " + typeName
			if trait := node.ChildByFieldName("trait"); trait != nil {
				sigStr = "impl " + text(trait, src) + " for " + typeName
			}
			syms = append(syms, sym(node, src, typeName, "impl", filePath, parent, "rust", sigStr, "", nil))
			if body := node.ChildByFieldName("body"); body != nil {
				for i := 0; i < nchildren(body); i++ {
					syms = append(syms, walkRust(child(body, i), src, filePath, typeName)...)
				}
			}
		}
	case "mod_item":
		if name := field(node, "name", src); name != "" {
			syms = append(syms, sym(node, src, name, "module", filePath, parent, "rust", "mod "+name, "", nil))
			if body := node.ChildByFieldName("body"); body != nil {
				for i := 0; i < nchildren(body); i++ {
					syms = append(syms, walkRust(child(body, i), src, filePath, name)...)
				}
			}
		}
	case "type_item":
		if name := field(node, "name", src); name != "" {
			syms = append(syms, sym(node, src, name, "type", filePath, parent, "rust", "type "+name, "", nil))
		}
	case "const_item":
		if name := field(node, "name", src); name != "" {
			syms = append(syms, sym(node, src, name, "constant", filePath, parent, "rust", "const "+name, "", nil))
		}
	default:
		for i := 0; i < nchildren(node); i++ {
			syms = append(syms, walkRust(child(node, i), src, filePath, parent)...)
		}
	}
	return syms
}

// ── Java ─────────────────────────────────────────────────────────────────────

func walkJava(node *sitter.Node, src []byte, filePath, parent string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	t := node.Type()
	switch t {
	case "class_declaration":
		if name := field(node, "name", src); name != "" {
			syms = append(syms, sym(node, src, name, "class", filePath, parent, "java", "class "+name, "", nil))
			if body := node.ChildByFieldName("body"); body != nil {
				for i := 0; i < nchildren(body); i++ {
					syms = append(syms, walkJava(child(body, i), src, filePath, name)...)
				}
			}
		}
	case "interface_declaration":
		if name := field(node, "name", src); name != "" {
			syms = append(syms, sym(node, src, name, "interface", filePath, parent, "java", "interface "+name, "", nil))
			if body := node.ChildByFieldName("body"); body != nil {
				for i := 0; i < nchildren(body); i++ {
					syms = append(syms, walkJava(child(body, i), src, filePath, name)...)
				}
			}
		}
	case "enum_declaration":
		if name := field(node, "name", src); name != "" {
			syms = append(syms, sym(node, src, name, "enum", filePath, parent, "java", "enum "+name, "", nil))
		}
	case "method_declaration", "constructor_declaration":
		if name := field(node, "name", src); name != "" {
			kind := "method"
			if t == "constructor_declaration" {
				kind = "constructor"
			}
			params := field(node, "parameters", src)
			calls := collectCalls(node.ChildByFieldName("body"), src)
			syms = append(syms, sym(node, src, name, kind, filePath, parent, "java", name+params, "", calls))
		}
	case "field_declaration":
		for i := 0; i < nchildren(node); i++ {
			ch := child(node, i)
			if ch.Type() == "variable_declarator" {
				if name := field(ch, "name", src); name != "" {
					syms = append(syms, sym(node, src, name, "variable", filePath, parent, "java", name, "", nil))
				}
			}
		}
	default:
		for i := 0; i < nchildren(node); i++ {
			syms = append(syms, walkJava(child(node, i), src, filePath, parent)...)
		}
	}
	return syms
}

// ── C / C++ ───────────────────────────────────────────────────────────────────

func walkC(node *sitter.Node, src []byte, filePath, parent string, isCPP bool) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	t := node.Type()
	switch t {
	case "function_definition":
		if name := cFuncName(node, src); name != "" {
			sigStr := firstLine(node, src)
			calls := collectCalls(node.ChildByFieldName("body"), src)
			lang := "c"
			if isCPP {
				lang = "cpp"
			}
			syms = append(syms, sym(node, src, name, "function", filePath, parent, lang, sigStr, "", calls))
		}
	case "struct_specifier", "union_specifier":
		if name := field(node, "name", src); name != "" {
			kind := "struct"
			if t == "union_specifier" {
				kind = "union"
			}
			syms = append(syms, sym(node, src, name, kind, filePath, parent, "c", kind+" "+name, "", nil))
		}
	case "class_specifier":
		if isCPP {
			if name := field(node, "name", src); name != "" {
				syms = append(syms, sym(node, src, name, "class", filePath, parent, "cpp", "class "+name, "", nil))
				if body := node.ChildByFieldName("body"); body != nil {
					for i := 0; i < nchildren(body); i++ {
						syms = append(syms, walkC(child(body, i), src, filePath, name, isCPP)...)
					}
				}
			}
		}
	case "namespace_definition":
		if isCPP {
			if name := field(node, "name", src); name != "" {
				syms = append(syms, sym(node, src, name, "namespace", filePath, parent, "cpp", "namespace "+name, "", nil))
				if body := node.ChildByFieldName("body"); body != nil {
					for i := 0; i < nchildren(body); i++ {
						syms = append(syms, walkC(child(body, i), src, filePath, name, isCPP)...)
					}
				}
			}
		}
	default:
		for i := 0; i < nchildren(node); i++ {
			syms = append(syms, walkC(child(node, i), src, filePath, parent, isCPP)...)
		}
	}
	return syms
}

func cFuncName(funcDef *sitter.Node, src []byte) string {
	decl := funcDef.ChildByFieldName("declarator")
	if decl == nil {
		return ""
	}
	var find func(*sitter.Node) string
	find = func(n *sitter.Node) string {
		if n == nil {
			return ""
		}
		switch n.Type() {
		case "identifier":
			return text(n, src)
		case "function_declarator":
			return find(n.ChildByFieldName("declarator"))
		case "pointer_declarator":
			return find(n.ChildByFieldName("declarator"))
		case "qualified_identifier":
			return text(n, src)
		}
		for i := 0; i < nchildren(n); i++ {
			if s := find(child(n, i)); s != "" {
				return s
			}
		}
		return ""
	}
	return find(decl)
}

func firstLine(node *sitter.Node, src []byte) string {
	s := text(node, src)
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return strings.TrimSpace(s)
}

// ── Ruby ─────────────────────────────────────────────────────────────────────

func walkRuby(node *sitter.Node, src []byte, filePath, parent string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	t := node.Type()
	switch t {
	case "method", "singleton_method":
		if name := field(node, "name", src); name != "" {
			params := ""
			if p := node.ChildByFieldName("parameters"); p != nil {
				params = text(p, src)
			}
			calls := collectCalls(node.ChildByFieldName("body"), src)
			syms = append(syms, sym(node, src, name, "function", filePath, parent, "ruby", "def "+name+params, "", calls))
			if body := node.ChildByFieldName("body"); body != nil {
				for i := 0; i < nchildren(body); i++ {
					syms = append(syms, walkRuby(child(body, i), src, filePath, name)...)
				}
			}
		}
	case "class":
		if name := field(node, "name", src); name != "" {
			syms = append(syms, sym(node, src, name, "class", filePath, parent, "ruby", "class "+name, "", nil))
			if body := node.ChildByFieldName("body"); body != nil {
				for i := 0; i < nchildren(body); i++ {
					syms = append(syms, walkRuby(child(body, i), src, filePath, name)...)
				}
			}
		}
	case "module":
		if name := field(node, "name", src); name != "" {
			syms = append(syms, sym(node, src, name, "module", filePath, parent, "ruby", "module "+name, "", nil))
			if body := node.ChildByFieldName("body"); body != nil {
				for i := 0; i < nchildren(body); i++ {
					syms = append(syms, walkRuby(child(body, i), src, filePath, name)...)
				}
			}
		}
	default:
		for i := 0; i < nchildren(node); i++ {
			syms = append(syms, walkRuby(child(node, i), src, filePath, parent)...)
		}
	}
	return syms
}

// ── Bash ─────────────────────────────────────────────────────────────────────

func walkBash(node *sitter.Node, src []byte, filePath string) []db.Symbol {
	if node == nil {
		return nil
	}
	var syms []db.Symbol
	if node.Type() == "function_definition" {
		name := field(node, "name", src)
		if name == "" {
			for i := 0; i < nchildren(node); i++ {
				ch := child(node, i)
				if ch.Type() == "word" {
					name = text(ch, src)
					break
				}
			}
		}
		if name != "" {
			calls := collectCalls(node.ChildByFieldName("body"), src)
			syms = append(syms, sym(node, src, name, "function", filePath, "", "bash", name+"()", "", calls))
		}
	} else {
		for i := 0; i < nchildren(node); i++ {
			syms = append(syms, walkBash(child(node, i), src, filePath)...)
		}
	}
	return syms
}
