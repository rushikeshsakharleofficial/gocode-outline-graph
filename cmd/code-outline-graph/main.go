package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

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
	start := time.Now()
	count, err := idx.IndexAll(projectPath)
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
	start := time.Now()

	// RemoveStale removes entries for deleted/moved files.
	removed, err := idx.RemoveStale(projectPath)
	if err != nil {
		errorf("prune error during update: %v", err)
	}

	// IndexAll will skip up-to-date files (unless --force).
	_ = f.force // future: pass force flag to indexer
	count, err := idx.IndexAll(projectPath)
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
		fmt.Fprintln(os.Stderr, "usage: code-outline-graph search <path> <query>")
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
		fmt.Fprintln(os.Stderr, "usage: code-outline-graph outline <path> <file>")
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

	s := search.New(database)

	// Get an overall symbol count and per-file breakdown via broad search.
	allSymbols, err := s.KeywordSearch("", 10000)
	if err != nil {
		errorf("status error: %v", err)
	}

	// Count unique files and accumulate per-file symbol counts.
	fileCount := map[string]int{}
	for _, sym := range allSymbols {
		fileCount[sym.FilePath]++
	}

	// Build a sorted top-files list (simple insertion into a top-10 slice).
	type fileStat struct {
		path  string
		count int
	}
	var topFiles []fileStat
	for fp, cnt := range fileCount {
		topFiles = append(topFiles, fileStat{fp, cnt})
	}
	// Sort descending by count (bubble sort is fine for <=10000 entries at status time).
	for i := 0; i < len(topFiles); i++ {
		for j := i + 1; j < len(topFiles); j++ {
			if topFiles[j].count > topFiles[i].count {
				topFiles[i], topFiles[j] = topFiles[j], topFiles[i]
			}
		}
	}
	if len(topFiles) > 10 {
		topFiles = topFiles[:10]
	}

	fmt.Printf("%sProject:%s  %s%s%s\n", colorBold, colorReset, colorCyan, projectPath, colorReset)
	fmt.Printf("%sDatabase:%s %s%s%s\n", colorBold, colorReset, colorDim, dbPath, colorReset)
	fmt.Printf("%sFiles indexed:%s  %s%d%s\n", colorBold, colorReset, colorGreen, len(fileCount), colorReset)
	fmt.Printf("%sTotal symbols:%s  %s%d%s\n", colorBold, colorReset, colorGreen, len(allSymbols), colorReset)

	if len(topFiles) > 0 {
		fmt.Printf("\n%sTop files by symbol count:%s\n", colorBold, colorReset)
		for _, fs := range topFiles {
			fmt.Printf("  %s%-50s%s %s(%d symbols)%s\n",
				colorCyan, fs.path, colorReset,
				colorDim, fs.count, colorReset)
		}
	}
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
	mcpServers["code-outline-graph"] = mcpServerEntry{
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
		label string
		path  string
	}

	targets := []configTarget{
		{
			label: "Claude Code",
			path:  filepath.Join(home, ".claude", "claude_desktop_config.json"),
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
		displayPath := t.path
		if strings.HasPrefix(displayPath, home) {
			displayPath = "~" + displayPath[len(home):]
		}

		errMsg := mergeConfigFile(t.path, binaryPath)
		if errMsg != "" {
			stderrf("  %s%-16s%s %s%s%s  %s✗ %s%s\n",
				colorBold, t.label+":", colorReset,
				colorDim, displayPath, colorReset,
				colorRed, errMsg, colorReset)
		} else {
			stderrf("  %s%-16s%s %s%s%s  %s✓%s\n",
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
	fmt.Fprintf(os.Stderr, `%scode-outline-graph%s — Go code indexing and search tool

%sUsage:%s
  code-outline-graph <command> [options]

%sCommands:%s
  %sbuild%s   <path>         Full index of a project
  %supdate%s  <path>         Incremental reindex (changed files only)
  %ssearch%s  <path> <query> Full-text search for symbols
  %soutline%s <path> <file>  List symbols in a file
  %sstatus%s  <path>         Show DB statistics
  %sserve%s                  Start MCP server on stdio
  %sprune%s   <path>         Remove stale index entries
  %sinstall%s [path]         Write MCP config files for editors
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
	case "version":
		fmt.Println("code-outline-graph version 1.0.0 (Go)")
	default:
		fmt.Fprintf(os.Stderr, "%sunknown command: %s%s\n", colorRed, command, colorReset)
		printUsage()
		os.Exit(1)
	}
}
