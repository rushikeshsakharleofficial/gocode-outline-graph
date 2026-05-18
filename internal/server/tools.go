package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gocode-outline-graph/internal/db"
	"gocode-outline-graph/internal/watcher"
)

// ---------------------------------------------------------------------------
// Tool schema types
// ---------------------------------------------------------------------------

// Tool describes a single MCP tool for the tools/list response.
type Tool struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	InputSchema ToolSchema `json:"inputSchema"`
}

// ToolSchema is the JSON Schema for a tool's arguments.
type ToolSchema struct {
	Type       string                `json:"type"`
	Properties map[string]PropSchema `json:"properties"`
	Required   []string              `json:"required,omitempty"`
}

// PropSchema describes a single property within a ToolSchema.
type PropSchema struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ---------------------------------------------------------------------------
// Tool registry
// ---------------------------------------------------------------------------

// allTools returns the full list of tool definitions for the tools/list response.
func allTools() []Tool {
	return []Tool{
		{
			Name:        "index_project",
			Description: "Index a project directory. Builds or refreshes the symbol index. Pass watch=true to start file watcher.",
			InputSchema: ToolSchema{
				Type: "object",
				Properties: map[string]PropSchema{
					"project_path": {Type: "string", Description: "Absolute or relative path to the project root"},
					"watch":        {Type: "boolean", Description: "Start a file watcher to auto-reindex on save"},
					"workers":      {Type: "integer", Description: "Number of parallel parse workers (default: min(4, cpu_count))"},
				},
				Required: []string{"project_path"},
			},
		},
		{
			Name:        "update_project",
			Description: "Reindex only changed files in the project (faster than index_project).",
			InputSchema: ToolSchema{
				Type: "object",
				Properties: map[string]PropSchema{
					"project_path": {Type: "string", Description: "Absolute or relative path to the project root"},
				},
				Required: []string{"project_path"},
			},
		},
		{
			Name:        "list_outline",
			Description: "List all symbols in a source file with their line ranges, kinds, and signatures.",
			InputSchema: ToolSchema{
				Type: "object",
				Properties: map[string]PropSchema{
					"file_path":    {Type: "string", Description: "Path to the source file"},
					"project_path": {Type: "string", Description: "Project root (used to locate the index)"},
				},
				Required: []string{"file_path"},
			},
		},
		{
			Name:        "get_symbol",
			Description: "Get metadata for a named symbol (kind, file, line range, signature, docstring).",
			InputSchema: ToolSchema{
				Type: "object",
				Properties: map[string]PropSchema{
					"name":         {Type: "string", Description: "Symbol name to look up"},
					"file_path":    {Type: "string", Description: "Restrict search to this file (optional)"},
					"project_path": {Type: "string", Description: "Project root"},
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "read_symbol_body",
			Description: "Read the full source lines of a named symbol from its file.",
			InputSchema: ToolSchema{
				Type: "object",
				Properties: map[string]PropSchema{
					"name":          {Type: "string", Description: "Symbol name"},
					"file_path":     {Type: "string", Description: "Source file containing the symbol"},
					"project_path":  {Type: "string", Description: "Project root"},
					"context_lines": {Type: "integer", Description: "Expand read range by N lines before start and after end (default 0)"},
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "get_line_range",
			Description: "Read an arbitrary range of lines from a file (1-indexed, inclusive).",
			InputSchema: ToolSchema{
				Type: "object",
				Properties: map[string]PropSchema{
					"file_path":    {Type: "string", Description: "Path to the file"},
					"start_line":   {Type: "integer", Description: "First line to read (1-indexed)"},
					"end_line":     {Type: "integer", Description: "Last line to read (1-indexed, inclusive)"},
					"project_path": {Type: "string", Description: "Project root"},
				},
				Required: []string{"file_path", "start_line", "end_line"},
			},
		},
		{
			Name:        "get_file_header",
			Description: "Get the first N lines of a file (shebang, imports, top-level constants).",
			InputSchema: ToolSchema{
				Type: "object",
				Properties: map[string]PropSchema{
					"file_path":    {Type: "string", Description: "Path to the file"},
					"lines":        {Type: "integer", Description: "Number of lines to return (default 50)"},
					"project_path": {Type: "string", Description: "Project root"},
				},
				Required: []string{"file_path"},
			},
		},
		{
			Name:        "find_by_keyword",
			Description: "Full-text / keyword search across symbol names and signatures. Optional filters narrow by kind, language, or file glob.",
			InputSchema: ToolSchema{
				Type: "object",
				Properties: map[string]PropSchema{
					"query":        {Type: "string", Description: "Search query (leave empty to match all when using filters)"},
					"limit":        {Type: "integer", Description: "Maximum results to return (default 20)"},
					"project_path": {Type: "string", Description: "Project root"},
					"kind":         {Type: "string", Description: "Optional symbol kind filter: function, class, method, struct, interface, type, constant, module, enum, decorator"},
					"language":     {Type: "string", Description: "Optional language filter (e.g. 'go', 'python', 'typescript')"},
					"file_pattern": {Type: "string", Description: "Optional SQLite GLOB pattern for file_path (e.g. '*/handlers/*.go')"},
				},
			},
		},
		{
			Name:        "get_outline_summary",
			Description: "Ultra-compressed project index summary: file count, symbol count, top files.",
			InputSchema: ToolSchema{
				Type: "object",
				Properties: map[string]PropSchema{
					"project_path": {Type: "string", Description: "Project root"},
				},
			},
		},
		{
			Name:        "prune_project",
			Description: "Remove stale index rows for files that have been deleted or are no longer indexable.",
			InputSchema: ToolSchema{
				Type: "object",
				Properties: map[string]PropSchema{
					"project_path": {Type: "string", Description: "Project root"},
				},
				Required: []string{"project_path"},
			},
		},
		{
			Name:        "find_callers",
			Description: "Find all symbols that call the named function.",
			InputSchema: ToolSchema{
				Type: "object",
				Properties: map[string]PropSchema{
					"name":         {Type: "string", Description: "Callee function name to search for"},
					"project_path": {Type: "string", Description: "Project root"},
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "find_callees",
			Description: "Find all functions called by the named symbol.",
			InputSchema: ToolSchema{
				Type: "object",
				Properties: map[string]PropSchema{
					"symbol_name":  {Type: "string", Description: "Caller symbol name"},
					"project_path": {Type: "string", Description: "Project root"},
				},
				Required: []string{"symbol_name"},
			},
		},
		{
			Name:        "list_files",
			Description: "List all indexed file paths, optionally filtered by a glob pattern.",
			InputSchema: ToolSchema{
				Type: "object",
				Properties: map[string]PropSchema{
					"project_path": {Type: "string", Description: "Project root"},
					"file_pattern": {Type: "string", Description: "Optional glob pattern (e.g. '*.go', '*/handlers/*')"},
				},
				Required: []string{"project_path"},
			},
		},
		{
			Name:        "find_by_kind",
			Description: "Find all symbols of a given kind (function, class, method, struct, interface, type, constant, module, enum, decorator) across the project. Returns at most 200 results.",
			InputSchema: ToolSchema{
				Type: "object",
				Properties: map[string]PropSchema{
					"kind":         {Type: "string", Description: "Symbol kind: function, class, method, struct, interface, type, constant, module, enum, decorator"},
					"project_path": {Type: "string", Description: "Project root"},
					"language":     {Type: "string", Description: "Optional language filter (e.g. 'go', 'python')"},
				},
				Required: []string{"kind", "project_path"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Tool dispatch
// ---------------------------------------------------------------------------

// callTool parses the tools/call params, dispatches to the appropriate handler,
// and returns either a result map[string]any or a *Response on error.
func callTool(params json.RawMessage) interface{} {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return &Response{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    errCodeInvalidParams,
				Message: "invalid params",
				Data:    err.Error(),
			},
		}
	}

	switch p.Name {
	case "index_project":
		return handleIndexProject(p.Arguments)
	case "update_project":
		return handleUpdateProject(p.Arguments)
	case "list_outline":
		return handleListOutline(p.Arguments)
	case "get_symbol":
		return handleGetSymbol(p.Arguments)
	case "read_symbol_body":
		return handleReadSymbolBody(p.Arguments)
	case "get_line_range":
		return handleGetLineRange(p.Arguments)
	case "get_file_header":
		return handleGetFileHeader(p.Arguments)
	case "find_by_keyword":
		return handleFindByKeyword(p.Arguments)
	case "get_outline_summary":
		return handleGetOutlineSummary(p.Arguments)
	case "prune_project":
		return handlePruneProject(p.Arguments)
	case "find_callers":
		return handleFindCallers(p.Arguments)
	case "find_callees":
		return handleFindCallees(p.Arguments)
	case "list_files":
		return handleListFiles(p.Arguments)
	case "find_by_kind":
		return handleFindByKind(p.Arguments)
	default:
		return &Response{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    errCodeMethodNotFound,
				Message: "unknown tool",
				Data:    p.Name,
			},
		}
	}
}

// ---------------------------------------------------------------------------
// Helper: wrap a plain text string in the MCP content envelope.
// ---------------------------------------------------------------------------

func textResult(text string) map[string]interface{} {
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": text},
		},
	}
}

// toolError wraps an error string in the MCP content envelope (error path is
// returned as tool content, not an RPC error, matching MCP convention).
func toolError(format string, args ...interface{}) map[string]interface{} {
	return textResult(fmt.Sprintf("error: "+format, args...))
}

// ---------------------------------------------------------------------------
// readLines reads lines [start, end] (1-indexed, inclusive) from filePath.
// Lines outside the file range are silently clamped.
// ---------------------------------------------------------------------------

func readLines(filePath string, start, end int) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if start < 1 {
		start = 1
	}
	if end < start {
		end = start
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		if lineNo < start {
			continue
		}
		if lineNo > end {
			break
		}
		sb.WriteString(scanner.Text())
		sb.WriteByte('\n')
	}
	return sb.String(), scanner.Err()
}

// readLinesNumbered is like readLines but prefixes each line with its number.
func readLinesNumbered(filePath string, start, end int) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if start < 1 {
		start = 1
	}
	if end < start {
		end = start
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		if lineNo < start {
			continue
		}
		if lineNo > end {
			break
		}
		fmt.Fprintf(&sb, "%d\t%s\n", lineNo, scanner.Text())
	}
	return sb.String(), scanner.Err()
}

// absPath expands ~ and makes a path absolute relative to cwd.
func absPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			p = filepath.Join(home, p[2:])
		}
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

// ---------------------------------------------------------------------------
// Tool handlers
// ---------------------------------------------------------------------------

// 1. index_project
func handleIndexProject(raw json.RawMessage) interface{} {
	var args struct {
		ProjectPath string `json:"project_path"`
		Watch       bool   `json:"watch"`
		Workers     int    `json:"workers"`
	}
	if raw != nil {
		json.Unmarshal(raw, &args) //nolint:errcheck — defaults on parse failure
	}

	if args.ProjectPath == "" {
		return toolError("project_path is required")
	}

	database, idx, _, err := GetComponents(args.ProjectPath)
	if err != nil {
		return toolError("init components: %v", err)
	}
	_ = database

	n, err := idx.IndexAll(args.ProjectPath)
	if err != nil {
		return toolError("index all: %v", err)
	}

	if args.Watch {
		stateMu.Lock()
		if globalWatcher != nil {
			globalWatcher.Stop()
		}
		w := watcher.New(idx, database)
		if werr := w.Start(args.ProjectPath); werr != nil {
			stateMu.Unlock()
			return toolError("start watcher: %v", werr)
		}
		globalWatcher = w
		stateMu.Unlock()
	}

	return textResult(fmt.Sprintf("Indexed %d files in %s", n, args.ProjectPath))
}

// 2. update_project
func handleUpdateProject(raw json.RawMessage) interface{} {
	var args struct {
		ProjectPath string `json:"project_path"`
	}
	if raw != nil {
		json.Unmarshal(raw, &args) //nolint:errcheck
	}

	if args.ProjectPath == "" {
		return toolError("project_path is required")
	}

	_, idx, _, err := GetComponents(args.ProjectPath)
	if err != nil {
		return toolError("init components: %v", err)
	}

	n, err := idx.IndexAll(args.ProjectPath)
	if err != nil {
		return toolError("index all: %v", err)
	}

	return textResult(fmt.Sprintf("Updated index for %s (%d files processed)", args.ProjectPath, n))
}

// 3. list_outline
func handleListOutline(raw json.RawMessage) interface{} {
	var args struct {
		FilePath    string `json:"file_path"`
		ProjectPath string `json:"project_path"`
	}
	if raw != nil {
		json.Unmarshal(raw, &args) //nolint:errcheck
	}

	if args.FilePath == "" {
		return toolError("file_path is required")
	}

	filePath := absPath(args.FilePath)

	projectPath := args.ProjectPath
	if projectPath == "" {
		projectPath = filepath.Dir(filePath)
	}

	database, idx, _, err := GetComponents(projectPath)
	if err != nil {
		return toolError("init components: %v", err)
	}

	if err := idx.EnsureFresh(filePath); err != nil {
		// Non-fatal: the file may simply not be indexed yet.
		fmt.Fprintf(os.Stderr, "list_outline: ensure_fresh %q: %v\n", filePath, err)
	}

	symbols, err := database.GetSymbolsForFile(filePath)
	if err != nil {
		return toolError("get symbols: %v", err)
	}

	if len(symbols) == 0 {
		return textResult(fmt.Sprintf("No symbols found in %s", filePath))
	}

	var sb strings.Builder
	for _, s := range symbols {
		fmt.Fprintf(&sb, "%s %s (lines %d-%d)\n", s.Kind, s.Name, s.StartLine, s.EndLine)
		if s.Signature != "" {
			fmt.Fprintf(&sb, "  %s\n", s.Signature)
		}
	}
	return textResult(sb.String())
}

// 4. get_symbol
func handleGetSymbol(raw json.RawMessage) interface{} {
	var args struct {
		Name        string `json:"name"`
		FilePath    string `json:"file_path"`
		ProjectPath string `json:"project_path"`
	}
	if raw != nil {
		json.Unmarshal(raw, &args) //nolint:errcheck
	}

	if args.Name == "" {
		return toolError("name is required")
	}

	filePath := ""
	if args.FilePath != "" {
		filePath = absPath(args.FilePath)
	}

	projectPath := args.ProjectPath
	if projectPath == "" && filePath != "" {
		projectPath = filepath.Dir(filePath)
	}
	if projectPath == "" {
		return toolError("project_path is required when file_path is not provided")
	}

	database, idx, _, err := GetComponents(projectPath)
	if err != nil {
		return toolError("init components: %v", err)
	}

	if filePath != "" {
		if err := idx.EnsureFresh(filePath); err != nil {
			fmt.Fprintf(os.Stderr, "get_symbol: ensure_fresh %q: %v\n", filePath, err)
		}
	}

	syms, err := database.GetSymbolsByName(args.Name, filePath)
	if err != nil {
		return toolError("get symbol: %v", err)
	}
	if len(syms) == 0 {
		return textResult(fmt.Sprintf(`{"error":"not_found","name":%q}`, args.Name))
	}

	symToMap := func(s db.Symbol) map[string]interface{} {
		return map[string]interface{}{
			"name":       s.Name,
			"kind":       s.Kind,
			"file_path":  s.FilePath,
			"start_line": s.StartLine,
			"end_line":   s.EndLine,
			"signature":  s.Signature,
			"docstring":  s.Docstring,
			"parent":     s.Parent,
			"language":   s.Language,
		}
	}

	// Single match or file_path specified → return object (backward compat).
	if len(syms) == 1 || filePath != "" {
		out, _ := json.Marshal(symToMap(syms[0]))
		return textResult(string(out))
	}

	// Multiple matches → return array for disambiguation.
	arr := make([]map[string]interface{}, len(syms))
	for i, s := range syms {
		arr[i] = symToMap(s)
	}
	out, _ := json.Marshal(arr)
	return textResult(string(out))
}

// 5. read_symbol_body
func handleReadSymbolBody(raw json.RawMessage) interface{} {
	var args struct {
		Name         string `json:"name"`
		FilePath     string `json:"file_path"`
		ProjectPath  string `json:"project_path"`
		ContextLines int    `json:"context_lines"`
	}
	if raw != nil {
		json.Unmarshal(raw, &args) //nolint:errcheck
	}

	if args.Name == "" {
		return toolError("name is required")
	}

	filePath := ""
	if args.FilePath != "" {
		filePath = absPath(args.FilePath)
	}

	projectPath := args.ProjectPath
	if projectPath == "" && filePath != "" {
		projectPath = filepath.Dir(filePath)
	}
	if projectPath == "" {
		return toolError("project_path is required when file_path is not provided")
	}

	database, idx, _, err := GetComponents(projectPath)
	if err != nil {
		return toolError("init components: %v", err)
	}

	if filePath != "" {
		if err := idx.EnsureFresh(filePath); err != nil {
			fmt.Fprintf(os.Stderr, "read_symbol_body: ensure_fresh %q: %v\n", filePath, err)
		}
	}

	sym, err := database.GetSymbolByName(args.Name, filePath)
	if err != nil {
		return toolError("get symbol: %v", err)
	}
	if sym == nil {
		return textResult(fmt.Sprintf(`{"error":"not_found","name":%q}`, args.Name))
	}

	// Prefer sym.FilePath if filePath was not specified.
	targetFile := sym.FilePath
	if targetFile == "" {
		targetFile = filePath
	}

	if args.ContextLines < 0 {
		args.ContextLines = 0
	}
	startLine := sym.StartLine - args.ContextLines
	if startLine < 1 {
		startLine = 1
	}
	endLine := sym.EndLine + args.ContextLines

	body, err := readLinesNumbered(targetFile, startLine, endLine)
	if err != nil {
		return toolError("read file: %v", err)
	}
	return textResult(body)
}

// 6. get_line_range
func handleGetLineRange(raw json.RawMessage) interface{} {
	var args struct {
		FilePath    string `json:"file_path"`
		StartLine   int    `json:"start_line"`
		EndLine     int    `json:"end_line"`
		ProjectPath string `json:"project_path"`
	}
	if raw != nil {
		json.Unmarshal(raw, &args) //nolint:errcheck
	}

	if args.FilePath == "" {
		return toolError("file_path is required")
	}

	filePath := absPath(args.FilePath)

	body, err := readLinesNumbered(filePath, args.StartLine, args.EndLine)
	if err != nil {
		return toolError("read file %q: %v", filePath, err)
	}
	return textResult(body)
}

// 7. get_file_header
func handleGetFileHeader(raw json.RawMessage) interface{} {
	var args struct {
		FilePath    string `json:"file_path"`
		Lines       int    `json:"lines"`
		ProjectPath string `json:"project_path"`
	}
	// Default
	args.Lines = 50
	if raw != nil {
		json.Unmarshal(raw, &args) //nolint:errcheck
	}
	if args.Lines <= 0 {
		args.Lines = 50
	}

	if args.FilePath == "" {
		return toolError("file_path is required")
	}

	filePath := absPath(args.FilePath)

	body, err := readLinesNumbered(filePath, 1, args.Lines)
	if err != nil {
		return toolError("read file %q: %v", filePath, err)
	}
	return textResult(body)
}

// 8. find_by_keyword
func handleFindByKeyword(raw json.RawMessage) interface{} {
	var args struct {
		Query       string `json:"query"`
		Limit       int    `json:"limit"`
		ProjectPath string `json:"project_path"`
		Kind        string `json:"kind"`
		Language    string `json:"language"`
		FilePattern string `json:"file_pattern"`
	}
	args.Limit = 20
	if raw != nil {
		json.Unmarshal(raw, &args) //nolint:errcheck
	}
	if args.Limit <= 0 {
		args.Limit = 20
	}

	if args.Query == "" && args.Kind == "" && args.Language == "" && args.FilePattern == "" {
		return toolError("query is required when no filters are specified")
	}

	projectPath := args.ProjectPath
	if projectPath == "" {
		return toolError("project_path is required")
	}

	database, _, _, err := GetComponents(projectPath)
	if err != nil {
		return toolError("init components: %v", err)
	}

	results, err := database.FilteredSearch(args.Query, args.Kind, args.Language, args.FilePattern, args.Limit)
	if err != nil {
		return toolError("search: %v", err)
	}

	if len(results) == 0 {
		if args.Query != "" {
			return textResult(fmt.Sprintf("No symbols found for query: %q", args.Query))
		}
		return textResult("No symbols found matching the specified filters")
	}

	var sb strings.Builder
	for _, s := range results {
		fmt.Fprintf(&sb, "%s:%d %s %s\n", s.FilePath, s.StartLine, s.Kind, s.Name)
		if s.Signature != "" {
			fmt.Fprintf(&sb, "  %s\n", s.Signature)
		}
	}
	return textResult(sb.String())
}

// 9. get_outline_summary
func handleGetOutlineSummary(raw json.RawMessage) interface{} {
	var args struct {
		ProjectPath string `json:"project_path"`
	}
	if raw != nil {
		json.Unmarshal(raw, &args) //nolint:errcheck
	}

	if args.ProjectPath == "" {
		return toolError("project_path is required")
	}

	database, _, _, err := GetComponents(args.ProjectPath)
	if err != nil {
		return toolError("init components: %v", err)
	}

	fileCount, symbolCount, err := database.GetFileStats()
	if err != nil {
		return toolError("get file stats: %v", err)
	}

	topFiles, err := database.GetTopFiles(10)
	if err != nil {
		return toolError("get top files: %v", err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Project: %s\n", args.ProjectPath)
	fmt.Fprintf(&sb, "Indexed files:  %d\n", fileCount)
	fmt.Fprintf(&sb, "Total symbols:  %d\n", symbolCount)
	if len(topFiles) > 0 {
		sb.WriteString("\nTop files by symbol count:\n")
		for i, f := range topFiles {
			fmt.Fprintf(&sb, "  %2d. %s\n", i+1, f)
		}
	}
	return textResult(sb.String())
}

// 10. prune_project
func handlePruneProject(raw json.RawMessage) interface{} {
	var args struct {
		ProjectPath string `json:"project_path"`
	}
	if raw != nil {
		json.Unmarshal(raw, &args) //nolint:errcheck
	}

	if args.ProjectPath == "" {
		return toolError("project_path is required")
	}

	_, idx, _, err := GetComponents(args.ProjectPath)
	if err != nil {
		return toolError("init components: %v", err)
	}

	removed, err := idx.RemoveStale(args.ProjectPath)
	if err != nil {
		return toolError("remove stale: %v", err)
	}

	return textResult(fmt.Sprintf("Removed %d stale entries from index", removed))
}

// 11. find_callers
func handleFindCallers(raw json.RawMessage) interface{} {
	var args struct {
		Name        string `json:"name"`
		ProjectPath string `json:"project_path"`
	}
	if raw != nil {
		json.Unmarshal(raw, &args) //nolint:errcheck
	}

	if args.Name == "" {
		return toolError("name is required")
	}

	projectPath := args.ProjectPath
	if projectPath == "" {
		return toolError("project_path is required")
	}

	database, _, _, err := GetComponents(projectPath)
	if err != nil {
		return toolError("init components: %v", err)
	}

	callers, err := database.GetCallersByName(args.Name)
	if err != nil {
		return toolError("get callers: %v", err)
	}

	if len(callers) == 0 {
		return textResult(fmt.Sprintf("No callers found for %q", args.Name))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Callers of %q:\n", args.Name)
	for _, s := range callers {
		fmt.Fprintf(&sb, "  %s:%d %s %s\n", s.FilePath, s.StartLine, s.Kind, s.Name)
		if s.Signature != "" {
			fmt.Fprintf(&sb, "    %s\n", s.Signature)
		}
	}
	return textResult(sb.String())
}

// 12. find_callees
func handleFindCallees(raw json.RawMessage) interface{} {
	var args struct {
		SymbolName  string `json:"symbol_name"`
		ProjectPath string `json:"project_path"`
	}
	if raw != nil {
		json.Unmarshal(raw, &args) //nolint:errcheck
	}

	if args.SymbolName == "" {
		return toolError("symbol_name is required")
	}

	projectPath := args.ProjectPath
	if projectPath == "" {
		return toolError("project_path is required")
	}

	database, _, _, err := GetComponents(projectPath)
	if err != nil {
		return toolError("init components: %v", err)
	}

	// Look up the symbol to get its ID.
	sym, err := database.GetSymbolByName(args.SymbolName, "")
	if err != nil {
		return toolError("get symbol: %v", err)
	}
	if sym == nil {
		return textResult(fmt.Sprintf(`{"error":"not_found","name":%q}`, args.SymbolName))
	}

	resolved, unresolved, err := database.GetCalleeSymbols(sym.ID)
	if err != nil {
		return toolError("get callees: %v", err)
	}

	if len(resolved) == 0 && len(unresolved) == 0 {
		return textResult(fmt.Sprintf("No callees found for %q", args.SymbolName))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Callees of %q:\n", args.SymbolName)
	for _, s := range resolved {
		fmt.Fprintf(&sb, "  %s:%d %s %s\n", s.FilePath, s.StartLine, s.Kind, s.Name)
		if s.Signature != "" {
			fmt.Fprintf(&sb, "    %s\n", s.Signature)
		}
	}
	for _, name := range unresolved {
		fmt.Fprintf(&sb, "  (unresolved) %s\n", name)
	}
	return textResult(sb.String())
}

// 13. list_files
func handleListFiles(raw json.RawMessage) interface{} {
	var args struct {
		ProjectPath string `json:"project_path"`
		FilePattern string `json:"file_pattern"`
	}
	if raw != nil {
		json.Unmarshal(raw, &args) //nolint:errcheck
	}

	if args.ProjectPath == "" {
		return toolError("project_path is required")
	}

	database, _, _, err := GetComponents(args.ProjectPath)
	if err != nil {
		return toolError("init components: %v", err)
	}

	files, err := database.ListIndexedFiles()
	if err != nil {
		return toolError("list files: %v", err)
	}

	if args.FilePattern != "" {
		// Validate pattern before iterating.
		if _, err := filepath.Match(args.FilePattern, ""); err != nil {
			return toolError("invalid file_pattern %q: %v", args.FilePattern, err)
		}
		var filtered []string
		for _, f := range files {
			// Try matching full path first.
			if matched, _ := filepath.Match(args.FilePattern, f); matched {
				filtered = append(filtered, f)
				continue
			}
			// Also match against base name for patterns like "*.go".
			if matched, _ := filepath.Match(args.FilePattern, filepath.Base(f)); matched {
				filtered = append(filtered, f)
			}
		}
		files = filtered
	}

	if len(files) == 0 {
		return textResult("No indexed files found")
	}
	return textResult(strings.Join(files, "\n"))
}

// 14. find_by_kind
func handleFindByKind(raw json.RawMessage) interface{} {
	var args struct {
		Kind        string `json:"kind"`
		ProjectPath string `json:"project_path"`
		Language    string `json:"language"`
	}
	if raw != nil {
		json.Unmarshal(raw, &args) //nolint:errcheck
	}

	if args.Kind == "" {
		return toolError("kind is required")
	}
	if args.ProjectPath == "" {
		return toolError("project_path is required")
	}

	database, _, _, err := GetComponents(args.ProjectPath)
	if err != nil {
		return toolError("init components: %v", err)
	}

	const maxResults = 200
	symbols, err := database.GetSymbolsByKind(args.Kind, args.Language, maxResults)
	if err != nil {
		return toolError("get symbols by kind: %v", err)
	}

	if len(symbols) == 0 {
		return textResult(fmt.Sprintf("No %q symbols found", args.Kind))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Symbols of kind %q (%d found):\n", args.Kind, len(symbols))
	for _, s := range symbols {
		fmt.Fprintf(&sb, "  %s:%d %s\n", s.FilePath, s.StartLine, s.Name)
		if s.Signature != "" {
			fmt.Fprintf(&sb, "    %s\n", s.Signature)
		}
	}
	return textResult(sb.String())
}
