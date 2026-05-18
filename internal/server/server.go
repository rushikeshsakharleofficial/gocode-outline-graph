// Package server implements a hand-rolled MCP (Model Context Protocol) server
// over JSON-RPC 2.0 using stdio transport (newline-delimited JSON).
package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"gocode-outline-graph/internal/db"
	"gocode-outline-graph/internal/indexer"
	"gocode-outline-graph/internal/paths"
	"gocode-outline-graph/internal/search"
	"gocode-outline-graph/internal/watcher"
)

// ---------------------------------------------------------------------------
// JSON-RPC 2.0 wire types
// ---------------------------------------------------------------------------

// Request is an incoming JSON-RPC 2.0 request or notification.
type Request struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

// Response is an outgoing JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Result  interface{}      `json:"result,omitempty"`
	Error   *RPCError        `json:"error,omitempty"`
}

// RPCError is a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

// Standard JSON-RPC 2.0 error codes.
const (
	errCodeParseError     = -32700
	errCodeInvalidRequest = -32600
	errCodeMethodNotFound = -32601
	errCodeInvalidParams  = -32602
	errCodeInternalError  = -32603
)

// ---------------------------------------------------------------------------
// Global lazy-initialized state
// ---------------------------------------------------------------------------

var (
	globalDB      *db.Database
	globalIndexer *indexer.Indexer
	globalSearcher *search.Searcher
	globalWatcher *watcher.CodeWatcher
	globalProject string
	stateMu       sync.Mutex
)

// ---------------------------------------------------------------------------
// Run — main entry point: reads JSON-RPC requests from stdin, writes to stdout
// ---------------------------------------------------------------------------

// Run starts the MCP stdio server loop. It reads newline-delimited JSON-RPC
// messages from stdin, routes them to the appropriate handler, and writes
// responses to stdout. It blocks until stdin is closed.
func Run() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024) // 10 MB buffer

	writer := bufio.NewWriter(os.Stdout)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		response := handleRequest(line)
		if response == nil {
			// Notifications have no response.
			continue
		}

		if err := json.NewEncoder(writer).Encode(response); err != nil {
			// Last-resort: write a plain error to stderr and continue.
			fmt.Fprintf(os.Stderr, "server: encode response: %v\n", err)
		}
		writer.Flush()
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "server: scanner error: %v\n", err)
	}
}

// ---------------------------------------------------------------------------
// Request routing
// ---------------------------------------------------------------------------

// handleRequest parses one newline-delimited JSON-RPC message and returns the
// response to send, or nil if no response should be sent (e.g. notifications).
func handleRequest(raw []byte) *Response {
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		return &Response{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    errCodeParseError,
				Message: "parse error",
				Data:    err.Error(),
			},
		}
	}

	// Validate JSON-RPC version.
	if req.JSONRPC != "2.0" {
		return errorResponse(req.ID, errCodeInvalidRequest, "invalid request", "jsonrpc must be '2.0'")
	}

	switch req.Method {
	case "initialize":
		return successResponse(req.ID, initResult())

	case "notifications/initialized":
		// Per spec: notifications never get a response.
		return nil

	case "tools/list":
		return successResponse(req.ID, map[string]interface{}{"tools": allTools()})

	case "tools/call":
		result := callTool(req.Params)
		// callTool may return a *Response (error) or a plain result map.
		switch v := result.(type) {
		case *Response:
			v.ID = req.ID
			return v
		default:
			return successResponse(req.ID, result)
		}

	case "ping":
		return successResponse(req.ID, map[string]interface{}{})

	default:
		// All other notification-style methods (prefix "notifications/") → no response.
		if len(req.Method) > 14 && req.Method[:14] == "notifications/" {
			return nil
		}
		return errorResponse(req.ID, errCodeMethodNotFound, "method not found", req.Method)
	}
}

// ---------------------------------------------------------------------------
// Response constructors
// ---------------------------------------------------------------------------

func successResponse(id *json.RawMessage, result interface{}) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

func errorResponse(id *json.RawMessage, code int, message, data string) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

// internalError is a convenience helper for tool handlers.
func internalError(id *json.RawMessage, err error) *Response {
	return errorResponse(id, errCodeInternalError, "internal error", err.Error())
}

// ---------------------------------------------------------------------------
// initialize result
// ---------------------------------------------------------------------------

func initResult() map[string]interface{} {
	return map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
		"serverInfo": map[string]interface{}{
			"name":    "code-outline-graph",
			"version": "1.1.0",
		},
	}
}

// ---------------------------------------------------------------------------
// Component lifecycle — lazy init / project switching
// ---------------------------------------------------------------------------

// getOrInitComponents ensures that the global DB, Indexer, and Searcher are
// initialised for projectPath. If the active project differs from projectPath,
// the existing resources are closed and new ones are created.
//
// It is safe to call from multiple goroutines; it acquires stateMu internally.
func getOrInitComponents(projectPath string) (*db.Database, *indexer.Indexer, *search.Searcher, error) {
	stateMu.Lock()
	defer stateMu.Unlock()

	resolved, err := paths.ResolveProjectPath(projectPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolve project path %q: %w", projectPath, err)
	}

	if globalDB != nil && globalProject == resolved {
		// Already initialised for this project.
		return globalDB, globalIndexer, globalSearcher, nil
	}

	// Tear down existing resources for the previous project.
	if globalWatcher != nil {
		globalWatcher.Stop()
		globalWatcher = nil
	}
	if globalDB != nil {
		_ = globalDB.Close()
		globalDB = nil
	}

	// Ensure the per-project DB directory exists.
	if err := paths.EnsureProjectDBDir(resolved); err != nil {
		return nil, nil, nil, fmt.Errorf("ensure project db dir: %w", err)
	}

	dbPath := paths.ProjectDBPath(resolved)
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open database %q: %w", dbPath, err)
	}

	idx := indexer.New(database, 0 /* use default workers */)
	srch := search.New(database)

	globalDB = database
	globalIndexer = idx
	globalSearcher = srch
	globalProject = resolved

	return database, idx, srch, nil
}

// GetComponents is the exported version of getOrInitComponents for use by
// tools.go and other packages that need access to the shared singletons.
func GetComponents(projectPath string) (*db.Database, *indexer.Indexer, *search.Searcher, error) {
	return getOrInitComponents(projectPath)
}
