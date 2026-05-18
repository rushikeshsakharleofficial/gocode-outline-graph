package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"

	"gocode-outline-graph/internal/db"
	"gocode-outline-graph/internal/indexer"
	"gocode-outline-graph/internal/paths"
	"gocode-outline-graph/internal/search"
	"gocode-outline-graph/internal/server"
)

// ANSI color codes — written to stderr for status messages, stdout for data.
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorRed    = "\033[31m"
	colorDim    = "\033[2m"
)

// flags holds parsed CLI flags shared across commands.
type flags struct {
	workers int
	watch   bool
	force   bool
}

// defaultWorkers returns min(4, numCPU).
func defaultWorkers() int {
	n := runtime.NumCPU()
	if n > 4 {
		return 4
	}
	return n
}

// parseFlags extracts --workers N, --watch, and --force from args.
// It returns remaining positional args and the parsed flags.
func parseFlags(args []string) ([]string, flags) {
	f := flags{workers: defaultWorkers()}
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--workers":
			if i+1 < len(args) {
				i++
				if n, err := strconv.Atoi(args[i]); err == nil && n > 0 {
					f.workers = n
				}
			}
		case "--watch":
			f.watch = true
		case "--force":
			f.force = true
		default:
			if strings.HasPrefix(args[i], "--workers=") {
				val := strings.TrimPrefix(args[i], "--workers=")
				if n, err := strconv.Atoi(val); err == nil && n > 0 {
					f.workers = n
				}
			} else {
				positional = append(positional, args[i])
			}
		}
	}
	return positional, f
}

// errorf prints a red-formatted error message to stderr and exits 1.
func errorf(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintf(os.Stderr, "%s%s%s\n", colorRed, msg, colorReset)
	os.Exit(1)
}

// stderr prints a status message to stderr (no newline suffix added automatically).
func stderrf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

// openProjectDB resolves the project path, opens (or creates) the DB, and returns
// both the resolved path and the open database handle.
func openProjectDB(rawPath string) (string, *db.Database) {
	projectPath, err := paths.ResolveProjectPath(rawPath)
	if err != nil {
		errorf("error resolving project path: %v", err)
	}
	if err := paths.EnsureProjectDBDir(projectPath); err != nil {
		errorf("error creating DB directory: %v", err)
	}
	dbPath := paths.ProjectDBPath(projectPath)
	database, err := db.Open(dbPath)
	if err != nil {
		errorf("error opening database: %v", err)
	}
	return projectPath, database
}

// -------------------------------------------------------------------------
// Progress bar
// -------------------------------------------------------------------------

// isTerminal reports whether f is an interactive terminal.
func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// attachProgress wires a terminal progress bar onto idx.OnProgress.
// Returns a flush func that must be called after IndexAll to print a final newline.
func attachProgress(idx *indexer.Indexer) func() {
	if !isTerminal(os.Stderr) {
		return func() {}
	}
	const barWidth = 30
	var mu sync.Mutex
	idx.OnProgress = func(done, total int, filePath string) {
		pct := 0.0
		if total > 0 {
			pct = float64(done) / float64(total)
		}
		filled := int(pct * float64(barWidth))
		if filled > barWidth {
			filled = barWidth
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		// Truncate filename to fit terminal — show only base name.
		name := filepath.Base(filePath)
		const nameWidth = 30
		if len(name) > nameWidth {
			name = "…" + name[len(name)-nameWidth+1:]
		}
		mu.Lock()
		fmt.Fprintf(os.Stderr, "\r  %s[%s]%s %d/%d  %s%-*s%s",
			colorDim, bar, colorReset, done, total,
			colorCyan, nameWidth, name, colorReset)
		mu.Unlock()
	}
	return func() { fmt.Fprintln(os.Stderr) }
}

// -------------------------------------------------------------------------
// Commands
// -------------------------------------------------------------------------

func cmdBuild(args []string) {
	positional, f := parseFlags(args)

	rawPath := "."
	if len(positional) > 0 {
		rawPath = positional[0]
	}

	projectPath, database := openProjectDB(rawPath)

	stderrf("%s✓ Indexing %s%s%s...%s\n", colorGreen, colorCyan, projectPath, colorGreen, colorReset)
	stderrf("%s  Workers: %d%s\n", colorDim, f.workers, colorReset)

	idx := indexer.New(database, f.workers)
	idx.Force = f.force
	flush := attachProgress(idx)
	start := time.Now()
	count, err := idx.IndexAll(projectPath)
	flush()
	if err != nil {
		errorf("indexing error: %v", err)
	}
	elapsed := time.Since(start)

	stderrf("%s  Indexed files and found %s%s%d symbols%s\n",
		colorGreen, colorBold, colorGreen, count, colorReset)
	stderrf("%s  Time: %.1fs%s\n", colorDim, elapsed.Seconds(), colorReset)

	if f.watch {
		stderrf("%s  Watching for changes (Ctrl-C to stop)...%s\n", colorDim, colorReset)
		// Block forever — the watcher runs in background goroutines started by IndexAll.
		select {}
	}
}

func cmdUpdate(args []string) {
	positional, f := parseFlags(args)

	rawPath := "."
	if len(positional) > 0 {
		rawPath = positional[0]
	}

	projectPath, database := openProjectDB(rawPath)

	stderrf("%s✓ Updating %s%s%s...%s\n", colorGreen, colorCyan, projectPath, colorGreen, colorReset)
	stderrf("%s  Workers: %d%s\n", colorDim, f.workers, colorReset)

	idx := indexer.New(database, f.workers)
	idx.Force = f.force
	start := time.Now()

	// RemoveStale removes entries for deleted/moved files.
	removed, err := idx.RemoveStale(projectPath)
	if err != nil {
		errorf("prune error during update: %v", err)
	}

	flush := attachProgress(idx)
	count, err := idx.IndexAll(projectPath)
	flush()
	if err != nil {
		errorf("indexing error: %v", err)
	}
	elapsed := time.Since(start)

	stderrf("%s  Reindexed %s%s%d symbols%s", colorGreen, colorBold, colorGreen, count, colorReset)
	if removed > 0 {
		stderrf("%s  (removed %d stale entries)%s", colorDim, removed, colorReset)
	}
	stderrf("\n%s  Time: %.1fs%s\n", colorDim, elapsed.Seconds(), colorReset)

	if f.watch {
		stderrf("%s  Watching for changes (Ctrl-C to stop)...%s\n", colorDim, colorReset)
		select {}
	}
}

func cmdSearch(args []string) {
	positional, _ := parseFlags(args)

	if len(positional) < 2 {
		fmt.Fprintln(os.Stderr, "usage: code-outline-graph-go search <path> <query>")
		os.Exit(1)
	}
	rawPath := positional[0]
	query := positional[1]

	_, database := openProjectDB(rawPath)
	s := search.New(database)

	results, err := s.FTSSearch(query, 20)
	if err != nil {
		// Fall back to keyword search on FTS failure.
		results, err = s.KeywordSearch(query, 20)
		if err != nil {
			errorf("search error: %v", err)
		}
	}

	fmt.Printf("%sResults for %q (%d found):%s\n", colorBold, query, len(results), colorReset)
	for _, sym := range results {
		fmt.Printf("  %s%s%s %s%s%s  %s%s:%d%s\n",
			colorYellow, sym.Kind, colorReset,
			colorBold, sym.Name, colorReset,
			colorCyan, sym.FilePath, sym.StartLine, colorReset)
		if sym.Signature != "" {
			fmt.Printf("    %s%s%s\n", colorDim, sym.Signature, colorReset)
		}
	}
}

func cmdOutline(args []string) {
	positional, _ := parseFlags(args)

	if len(positional) < 2 {
		fmt.Fprintln(os.Stderr, "usage: code-outline-graph-go outline <path> <file>")
		os.Exit(1)
	}
	rawPath := positional[0]
	filePath := positional[1]

	absFile, err := filepath.Abs(filePath)
	if err != nil {
		absFile = filePath
	}

	_, database := openProjectDB(rawPath)

	symbols, err := database.GetSymbolsForFile(absFile)
	if err != nil {
		errorf("outline error: %v", err)
	}

	fmt.Printf("%sOutline: %s%s%s%s\n", colorBold, colorCyan, absFile, colorReset, colorReset)
	if len(symbols) == 0 {
		fmt.Printf("  %s(no symbols found)%s\n", colorDim, colorReset)
		return
	}
	for _, sym := range symbols {
		fmt.Printf("  %s%-12s%s %s%s%s  line %s%d-%d%s\n",
			colorYellow, sym.Kind, colorReset,
			colorBold, sym.Name, colorReset,
			colorDim, sym.StartLine, sym.EndLine, colorReset)
		if sym.Signature != "" {
			fmt.Printf("    %s%s%s\n", colorDim, sym.Signature, colorReset)
		}
	}
}

func cmdStatus(args []string) {
	positional, _ := parseFlags(args)

	rawPath := "."
	if len(positional) > 0 {
		rawPath = positional[0]
	}

	projectPath, database := openProjectDB(rawPath)
	dbPath := paths.ProjectDBPath(projectPath)

	fileCount, symbolCount, err := database.GetFileStats()
	if err != nil {
		errorf("status error: %v", err)
	}

	topFiles, err := database.GetTopFilesWithCounts(10)
	if err != nil {
		errorf("status error: %v", err)
	}

	fmt.Printf("%sProject:%s  %s%s%s\n", colorBold, colorReset, colorCyan, projectPath, colorReset)
	fmt.Printf("%sDatabase:%s %s%s%s\n", colorBold, colorReset, colorDim, dbPath, colorReset)
	fmt.Printf("%sFiles indexed:%s  %s%d%s\n", colorBold, colorReset, colorGreen, fileCount, colorReset)
	fmt.Printf("%sTotal symbols:%s  %s%d%s\n", colorBold, colorReset, colorGreen, symbolCount, colorReset)

	if len(topFiles) > 0 {
		fmt.Printf("\n%sTop files by symbol count:%s\n", colorBold, colorReset)
		for _, f := range topFiles {
			fmt.Printf("  %s%-50s%s %s(%d symbols)%s\n",
				colorCyan, f.Path, colorReset,
				colorDim, f.Count, colorReset)
		}
	}
}

// skillContent is the SKILL.md embedded at build time.
// Users run `code-outline-graph install-skill` to install it.
const skillContent = `---
name: code-outline-graph-go
description: Use when working in any indexed codebase — before reading files, searching for symbols, tracing call chains, or understanding code structure. Prevents wasteful whole-file reads.
---

# code-outline-graph-go

Symbol index over source code. Find and read only what you need — 10x–50x fewer tokens than reading files.

## When to Use vs. When NOT to Use

**DO NOT use when:**
- File is already loaded in this conversation
- File is non-source: ` + "`.json`, `.yaml`, `.html`, `.css`, `.md`, `.sql`" + ` — not indexed
- You need runtime behavior, not code structure
- You just wrote a new file — run ` + "`update_project`" + ` first, then use tools
- Simple grep (exact string match) is faster than a symbol lookup

**ALWAYS use when:**
- About to open any source file → find the symbol first, read only its body
- Tracing who calls a function, or what a function calls
- Getting a project overview before diving in
- Locating all symbols of a given kind (all interfaces, all structs)

---

## The One Rule

` + "```" + `
About to Read a file? STOP.
find_by_keyword → confirm signature → read_symbol_body
` + "```" + `

---

## Tool Decision Table

| Situation | Tool |
|-----------|------|
| Know exact name | ` + "`get_symbol`" + ` |
| Know approximate concept | ` + "`find_by_keyword`" + ` |
| All symbols of one kind (functions, structs, interfaces) | ` + "`find_by_kind`" + ` |
| All symbols in a file | ` + "`list_outline`" + ` |
| All indexed files | ` + "`list_files`" + ` |
| Imports / file header | ` + "`get_file_header`" + ` |
| Read one function body | ` + "`read_symbol_body`" + ` |
| Read body + surrounding context | ` + "`read_symbol_body`" + ` with ` + "`context_lines: 5`" + ` |
| Read arbitrary lines | ` + "`get_line_range`" + ` |
| Who calls function X? | ` + "`find_callers`" + ` |
| What does function X call? | ` + "`find_callees`" + ` |
| Project overview (file count, top files) | ` + "`get_outline_summary`" + ` |
| First time using a project | ` + "`index_project`" + ` |
| After editing files | ` + "`update_project`" + ` |
| Files disappeared from disk | ` + "`prune_project`" + ` |

---

## Power Patterns

### Find with precision (use filters!)
` + "```" + `
find_by_keyword({
  "query": "auth",
  "kind": "function",
  "language": "go",
  "file_pattern": "*/handlers/*",
  "project_path": "."
})
` + "```" + `

### Multi-match disambiguation
` + "```" + `
get_symbol({"name": "Handler", "project_path": "."})
// returns array when multiple packages define Handler
// returns object when unique (backward compat)
` + "```" + `

### Read with context
` + "```" + `
read_symbol_body({"name": "Login", "file_path": "auth.go",
                  "context_lines": 5, "project_path": "."})
` + "```" + `

### Impact analysis before editing
` + "```" + `
find_callers({"name": "InsertSymbolsForFile", "project_path": "."})
→ every caller with file:line — know scope before touching anything
` + "```" + `

### Discover all types / interfaces
` + "```" + `
find_by_kind({"kind": "interface", "language": "go", "project_path": "."})
→ all interfaces (max 200)
` + "```" + `

### Call chain trace
` + "```" + `
find_callees({"symbol_name": "IndexAll", "project_path": "."})
→ resolved: file:line for each internal callee
→ unresolved: external/stdlib calls listed separately
` + "```" + `

---

## After Every Edit

` + "```" + `
update_project({"project_path": "."})
` + "```" + `

Without this, the index is stale and tools return old results.

---

## Supported Languages

python, javascript, typescript, tsx, go, rust, java, c, cpp, ruby, bash

Other extensions (.json, .yaml, .html, etc.) are NOT indexed — use Read directly for those.
` + "`"

func cmdInstallSkill(_ []string) {
	home, err := os.UserHomeDir()
	if err != nil {
		errorf("could not determine home directory: %v", err)
	}
	skillDir := filepath.Join(home, ".claude", "skills", "code-outline-graph-go")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		errorf("mkdir %q: %v", skillDir, err)
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(skillContent), 0o644); err != nil {
		errorf("write skill: %v", err)
	}
	stderrf("%s✓ Skill installed:%s %s%s%s\n", colorGreen, colorReset, colorCyan, skillPath, colorReset)
}

func cmdServe(args []string) {
	// stdout is reserved for MCP JSON — only write to stderr before starting.
	fmt.Fprintln(os.Stderr, colorDim+"Starting MCP server..."+colorReset)
	server.Run()
}

func cmdPrune(args []string) {
	positional, _ := parseFlags(args)

	rawPath := "."
	if len(positional) > 0 {
		rawPath = positional[0]
	}

	projectPath, database := openProjectDB(rawPath)
	idx := indexer.New(database, defaultWorkers())

	stderrf("%s✓ Pruning stale entries from %s%s%s...%s\n",
		colorGreen, colorCyan, projectPath, colorGreen, colorReset)

	removed, err := idx.RemoveStale(projectPath)
	if err != nil {
		errorf("prune error: %v", err)
	}

	stderrf("%s  Removed %d stale entries.%s\n", colorGreen, removed, colorReset)
}

// -------------------------------------------------------------------------
// install command helpers
// -------------------------------------------------------------------------

// mcpServerEntry is the JSON structure for a single MCP server definition.
type mcpServerEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// mergeConfigFile reads an existing JSON config file (if any), merges the
// mcpServers key with the provided entry, and writes the result back.
// Returns an error string on failure, or "" on success.
func mergeConfigFile(configPath string, binaryPath string) string {
	// Read existing file, if present.
	raw, err := os.ReadFile(configPath)

	var root map[string]interface{}
	if err == nil && len(raw) > 0 {
		if jsonErr := json.Unmarshal(raw, &root); jsonErr != nil {
			// Malformed JSON — start fresh.
			root = map[string]interface{}{}
		}
	} else {
		root = map[string]interface{}{}
	}

	// Retrieve or create mcpServers map.
	var mcpServers map[string]interface{}
	if existing, ok := root["mcpServers"]; ok {
		if m, ok := existing.(map[string]interface{}); ok {
			mcpServers = m
		} else {
			mcpServers = map[string]interface{}{}
		}
	} else {
		mcpServers = map[string]interface{}{}
	}

	// Overwrite our server entry.
	mcpServers["code-outline-graph-go"] = mcpServerEntry{
		Command: binaryPath,
		Args:    []string{"serve"},
		Env:     map[string]string{},
	}
	root["mcpServers"] = mcpServers

	// Marshal with indentation.
	out, marshalErr := json.MarshalIndent(root, "", "  ")
	if marshalErr != nil {
		return fmt.Sprintf("marshal error: %v", marshalErr)
	}

	// Ensure parent directory exists.
	if mkdirErr := os.MkdirAll(filepath.Dir(configPath), 0o755); mkdirErr != nil {
		return fmt.Sprintf("mkdir error: %v", mkdirErr)
	}

	if writeErr := os.WriteFile(configPath, out, 0o644); writeErr != nil {
		return fmt.Sprintf("write error: %v", writeErr)
	}
	return ""
}

// claudeDesktopPath returns the platform-appropriate path for the Claude Desktop
// app config file.
func claudeDesktopPath() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	}
	// Linux and others.
	return filepath.Join(home, ".config", "claude", "claude_desktop_config.json")
}

func cmdInstall(args []string) {
	positional, _ := parseFlags(args)

	// Project path is optional for install; we resolve it but don't open a DB.
	_ = positional // project path not needed for config file writing

	binaryPath, err := os.Executable()
	if err != nil {
		errorf("could not determine executable path: %v", err)
	}
	// Resolve any symlinks so we store the real binary location.
	if resolved, err := filepath.EvalSymlinks(binaryPath); err == nil {
		binaryPath = resolved
	}

	home, err := os.UserHomeDir()
	if err != nil {
		errorf("could not determine home directory: %v", err)
	}

	type configTarget struct {
		label   string
		path    string
		command string // override binary path; defaults to binaryPath
	}

	// Resolve project path for the local .mcp.json target.
	projectMCPPath := ""
	if rawPath := "."; true {
		if abs, err2 := filepath.Abs(rawPath); err2 == nil {
			projectMCPPath = filepath.Join(abs, ".mcp.json")
		}
	}

	targets := []configTarget{
		{
			label: "Claude Code",
			path:  filepath.Join(home, ".claude", "mcp.json"),
		},
		{
			label:   "Project .mcp.json",
			path:    projectMCPPath,
			command: "code-outline-graph-go", // bare name — portable across machines
		},
		{
			label: "Cursor",
			path:  filepath.Join(home, ".cursor", "mcp.json"),
		},
		{
			label: "Claude Desktop",
			path:  claudeDesktopPath(),
		},
	}

	stderrf("%s✓ Installed MCP server config%s\n", colorGreen, colorReset)
	stderrf("  %sBinary:%s %s%s%s\n\n", colorBold, colorReset, colorCyan, binaryPath, colorReset)

	for _, t := range targets {
		cmd := binaryPath
		if t.command != "" {
			cmd = t.command
		}
		displayPath := t.path
		if strings.HasPrefix(displayPath, home) {
			displayPath = "~" + displayPath[len(home):]
		}

		errMsg := mergeConfigFile(t.path, cmd)
		if errMsg != "" {
			stderrf("  %s%-20s%s %s%s%s  %s✗ %s%s\n",
				colorBold, t.label+":", colorReset,
				colorDim, displayPath, colorReset,
				colorRed, errMsg, colorReset)
		} else {
			stderrf("  %s%-20s%s %s%s%s  %s✓%s\n",
				colorBold, t.label+":", colorReset,
				colorDim, displayPath, colorReset,
				colorGreen, colorReset)
		}
	}
}

// -------------------------------------------------------------------------
// Usage
// -------------------------------------------------------------------------

func printUsage() {
	fmt.Fprintf(os.Stderr, `%scode-outline-graph-go%s — Go code indexing and search tool

%sUsage:%s
  code-outline-graph-go <command> [options]

%sCommands:%s
  %sbuild%s   <path>         Full index of a project
  %supdate%s  <path>         Incremental reindex (changed files only)
  %ssearch%s  <path> <query> Full-text search for symbols
  %soutline%s <path> <file>  List symbols in a file
  %sstatus%s  <path>         Show DB statistics
  %sserve%s                  Start MCP server on stdio
  %sprune%s   <path>         Remove stale index entries
  %sinstall%s [path]         Write MCP config files for editors
  %sinstall-skill%s          Install Claude Code skill to ~/.claude/skills/
  %sversion%s                Print version

%sOptions:%s
  --workers N   Number of parallel workers (default: min(4, numCPU))
  --watch       Watch for file changes after indexing (build/update only)
  --force       Force reindex even if files are current

Default path is "." if not provided.
`,
		colorBold, colorReset,
		colorBold, colorReset,
		colorBold, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorBold, colorReset,
	)
}

// -------------------------------------------------------------------------
// main
// -------------------------------------------------------------------------

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "build":
		cmdBuild(os.Args[2:])
	case "update":
		cmdUpdate(os.Args[2:])
	case "search":
		cmdSearch(os.Args[2:])
	case "outline":
		cmdOutline(os.Args[2:])
	case "status":
		cmdStatus(os.Args[2:])
	case "serve":
		cmdServe(os.Args[2:])
	case "prune":
		cmdPrune(os.Args[2:])
	case "install":
		cmdInstall(os.Args[2:])
	case "install-skill":
		cmdInstallSkill(os.Args[2:])
	case "version":
		fmt.Println("code-outline-graph-go version 1.1.1 (Go)")
	default:
		fmt.Fprintf(os.Stderr, "%sunknown command: %s%s\n", colorRed, command, colorReset)
		printUsage()
		os.Exit(1)
	}
}
