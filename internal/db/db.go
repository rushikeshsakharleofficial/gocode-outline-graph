// Package db defines shared types and all SQLite persistence operations.
package db

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Symbol represents a code symbol extracted from a source file.
// Calls is populated during parsing only and is NOT a DB column.
type Symbol struct {
	ID        int64
	Name      string
	Kind      string   // "function", "class", "method", "variable", "struct", "interface", "type", "constant", "module", "enum", "decorator"
	FilePath  string
	StartLine int      // 1-indexed
	EndLine   int      // 1-indexed
	StartByte int
	EndByte   int
	Signature string
	Docstring string
	Parent    string
	Language  string
	Calls     []string // callee names — parse-time only
}

// IndexedFile records the last-indexed state of a source file.
type IndexedFile struct {
	FilePath  string
	Checksum  string
	MtimeNs   int64
	FileSize  int64
	IndexedAt int64
}

// Database wraps a SQLite connection and provides all persistence operations.
type Database struct {
	db   *sql.DB
	mu   sync.Mutex
	path string
}

const symbolCols = `id, name, kind, file_path, start_line, end_line, start_byte, end_byte, signature, docstring, parent, language`

// Open opens (or creates) the SQLite DB at dbPath, applies schema, returns ready Database.
func Open(dbPath string) (*Database, error) {
	dsn := fmt.Sprintf(
		"file:%s?_journal_mode=WAL&_cache_size=-65536&_foreign_keys=on&_busy_timeout=5000",
		dbPath,
	)
	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite db at %q: %w", dbPath, err)
	}
	sqlDB.SetMaxOpenConns(1)

	d := &Database{db: sqlDB, path: dbPath}
	if err := d.createSchema(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}
	return d, nil
}

// Close releases the database connection.
func (d *Database) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.Close()
}

func (d *Database) createSchema() error {
	stmts := []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA cache_size=-65536`,
		`CREATE TABLE IF NOT EXISTS symbols (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT NOT NULL,
			kind       TEXT NOT NULL,
			file_path  TEXT NOT NULL,
			start_line INTEGER NOT NULL,
			end_line   INTEGER NOT NULL,
			start_byte INTEGER NOT NULL,
			end_byte   INTEGER NOT NULL,
			signature  TEXT DEFAULT '',
			docstring  TEXT DEFAULT '',
			parent     TEXT DEFAULT '',
			language   TEXT DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_symbols_file_path ON symbols(file_path)`,
		`CREATE INDEX IF NOT EXISTS idx_symbols_name ON symbols(name)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS symbols_fts USING fts5(
			name, kind, file_path, signature, docstring, parent,
			content=symbols, content_rowid=id,
			tokenize='porter unicode61'
		)`,
		`CREATE TABLE IF NOT EXISTS indexed_files (
			file_path  TEXT PRIMARY KEY,
			checksum   TEXT NOT NULL,
			mtime_ns   INTEGER NOT NULL,
			file_size  INTEGER NOT NULL,
			indexed_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS call_graph (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			caller_id   INTEGER NOT NULL,
			callee_name TEXT NOT NULL,
			FOREIGN KEY (caller_id) REFERENCES symbols(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_call_graph_caller ON call_graph(caller_id)`,
		`CREATE INDEX IF NOT EXISTS idx_call_graph_callee ON call_graph(callee_name)`,
		`CREATE TRIGGER IF NOT EXISTS symbols_ai AFTER INSERT ON symbols BEGIN
			INSERT INTO symbols_fts(rowid, name, kind, file_path, signature, docstring, parent)
			VALUES (new.id, new.name, new.kind, new.file_path, new.signature, new.docstring, new.parent);
		END`,
		`CREATE TRIGGER IF NOT EXISTS symbols_ad AFTER DELETE ON symbols BEGIN
			INSERT INTO symbols_fts(symbols_fts, rowid, name, kind, file_path, signature, docstring, parent)
			VALUES ('delete', old.id, old.name, old.kind, old.file_path, old.signature, old.docstring, old.parent);
		END`,
		`CREATE TRIGGER IF NOT EXISTS symbols_au AFTER UPDATE ON symbols BEGIN
			INSERT INTO symbols_fts(symbols_fts, rowid, name, kind, file_path, signature, docstring, parent)
			VALUES ('delete', old.id, old.name, old.kind, old.file_path, old.signature, old.docstring, old.parent);
			INSERT INTO symbols_fts(rowid, name, kind, file_path, signature, docstring, parent)
			VALUES (new.id, new.name, new.kind, new.file_path, new.signature, new.docstring, new.parent);
		END`,
	}
	for _, stmt := range stmts {
		if _, err := d.db.Exec(stmt); err != nil {
			short := stmt
			if len(short) > 60 {
				short = short[:60]
			}
			return fmt.Errorf("DDL %q: %w", short, err)
		}
	}
	return nil
}

func scanSymbol(rows *sql.Rows) (Symbol, error) {
	var s Symbol
	err := rows.Scan(&s.ID, &s.Name, &s.Kind, &s.FilePath,
		&s.StartLine, &s.EndLine, &s.StartByte, &s.EndByte,
		&s.Signature, &s.Docstring, &s.Parent, &s.Language)
	return s, err
}

func scanSymbolRow(row *sql.Row) (*Symbol, error) {
	var s Symbol
	err := row.Scan(&s.ID, &s.Name, &s.Kind, &s.FilePath,
		&s.StartLine, &s.EndLine, &s.StartByte, &s.EndByte,
		&s.Signature, &s.Docstring, &s.Parent, &s.Language)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func collectSymbols(rows *sql.Rows) ([]Symbol, error) {
	var syms []Symbol
	for rows.Next() {
		s, err := scanSymbol(rows)
		if err != nil {
			return nil, err
		}
		syms = append(syms, s)
	}
	return syms, rows.Err()
}

// InsertSymbolsForFile atomically replaces all symbols for filePath.
func (d *Database) InsertSymbolsForFile(filePath string, symbols []Symbol, checksum string, mtimeNs, fileSize int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	if _, err = tx.Exec(`DELETE FROM symbols WHERE file_path = ?`, filePath); err != nil {
		return fmt.Errorf("delete symbols for %q: %w", filePath, err)
	}

	insertSym, err := tx.Prepare(`INSERT INTO symbols
		(name, kind, file_path, start_line, end_line, start_byte, end_byte, signature, docstring, parent, language)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare symbol insert: %w", err)
	}
	defer insertSym.Close()

	insertCall, err := tx.Prepare(`INSERT INTO call_graph (caller_id, callee_name) VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare call_graph insert: %w", err)
	}
	defer insertCall.Close()

	for _, sym := range symbols {
		res, execErr := insertSym.Exec(
			sym.Name, sym.Kind, sym.FilePath,
			sym.StartLine, sym.EndLine, sym.StartByte, sym.EndByte,
			sym.Signature, sym.Docstring, sym.Parent, sym.Language,
		)
		if execErr != nil {
			err = fmt.Errorf("insert symbol %q: %w", sym.Name, execErr)
			return err
		}
		symID, idErr := res.LastInsertId()
		if idErr != nil {
			err = fmt.Errorf("last insert id: %w", idErr)
			return err
		}
		for _, callee := range sym.Calls {
			if _, execErr = insertCall.Exec(symID, callee); execErr != nil {
				err = fmt.Errorf("insert call_graph: %w", execErr)
				return err
			}
		}
	}

	now := time.Now().Unix()
	_, err = tx.Exec(`INSERT INTO indexed_files (file_path, checksum, mtime_ns, file_size, indexed_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(file_path) DO UPDATE SET
			checksum=excluded.checksum, mtime_ns=excluded.mtime_ns,
			file_size=excluded.file_size, indexed_at=excluded.indexed_at`,
		filePath, checksum, mtimeNs, fileSize, now)
	if err != nil {
		return fmt.Errorf("upsert indexed_files: %w", err)
	}
	err = tx.Commit()
	return err
}

// DeleteSymbolsForFile removes all symbols and indexed_files record for filePath.
func (d *Database) DeleteSymbolsForFile(filePath string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()
	if _, err = tx.Exec(`DELETE FROM symbols WHERE file_path = ?`, filePath); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM indexed_files WHERE file_path = ?`, filePath); err != nil {
		return err
	}
	err = tx.Commit()
	return err
}

// DeleteFile is an alias for DeleteSymbolsForFile.
func (d *Database) DeleteFile(filePath string) error {
	return d.DeleteSymbolsForFile(filePath)
}

// GetFileInfo returns the IndexedFile record or nil if not indexed.
func (d *Database) GetFileInfo(filePath string) (*IndexedFile, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var f IndexedFile
	err := d.db.QueryRow(`SELECT file_path, checksum, mtime_ns, file_size, indexed_at
		FROM indexed_files WHERE file_path = ?`, filePath).
		Scan(&f.FilePath, &f.Checksum, &f.MtimeNs, &f.FileSize, &f.IndexedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// IsFileCurrent returns true when stored (file_size, mtime_ns) match supplied values.
func (d *Database) IsFileCurrent(filePath string, size, mtimeNs int64) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	var storedSize, storedMtime int64
	err := d.db.QueryRow(`SELECT file_size, mtime_ns FROM indexed_files WHERE file_path = ?`, filePath).
		Scan(&storedSize, &storedMtime)
	if err != nil {
		return false
	}
	return storedSize == size && storedMtime == mtimeNs
}

// GetChecksumForFile returns stored checksum or "" if not indexed.
func (d *Database) GetChecksumForFile(filePath string) string {
	d.mu.Lock()
	defer d.mu.Unlock()

	var checksum string
	d.db.QueryRow(`SELECT checksum FROM indexed_files WHERE file_path = ?`, filePath).Scan(&checksum)
	return checksum
}

// GetSymbolsForFile returns all symbols for a file ordered by start_line.
func (d *Database) GetSymbolsForFile(filePath string) ([]Symbol, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rows, err := d.db.Query(`SELECT `+symbolCols+` FROM symbols WHERE file_path = ? ORDER BY start_line`, filePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSymbols(rows)
}

// GetSymbolByName finds first symbol with the given name, optionally filtered by file.
func (d *Database) GetSymbolByName(name, filePath string) (*Symbol, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if filePath != "" {
		return scanSymbolRow(d.db.QueryRow(`SELECT `+symbolCols+` FROM symbols WHERE name = ? AND file_path = ? LIMIT 1`, name, filePath))
	}
	return scanSymbolRow(d.db.QueryRow(`SELECT `+symbolCols+` FROM symbols WHERE name = ? LIMIT 1`, name))
}

// GetSymbolsByName returns all symbols with the given name, optionally filtered by file.
func (d *Database) GetSymbolsByName(name, filePath string) ([]Symbol, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var rows *sql.Rows
	var err error
	if filePath != "" {
		rows, err = d.db.Query(
			`SELECT `+symbolCols+` FROM symbols WHERE name = ? AND file_path = ? ORDER BY start_line`,
			name, filePath)
	} else {
		rows, err = d.db.Query(
			`SELECT `+symbolCols+` FROM symbols WHERE name = ? ORDER BY file_path, start_line`,
			name)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSymbols(rows)
}

// GetSymbolsByKind returns all symbols of a given kind, optionally filtered by language.
func (d *Database) GetSymbolsByKind(kind, language string, limit int) ([]Symbol, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var rows *sql.Rows
	var err error
	if language != "" {
		rows, err = d.db.Query(
			`SELECT `+symbolCols+` FROM symbols WHERE kind = ? AND language = ? ORDER BY file_path, start_line LIMIT ?`,
			kind, language, limit)
	} else {
		rows, err = d.db.Query(
			`SELECT `+symbolCols+` FROM symbols WHERE kind = ? ORDER BY file_path, start_line LIMIT ?`,
			kind, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSymbols(rows)
}

// GetFileStats returns indexed file count and total symbol count.
func (d *Database) GetFileStats() (fileCount, symbolCount int, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err = d.db.QueryRow(`SELECT COUNT(*) FROM indexed_files`).Scan(&fileCount); err != nil {
		return
	}
	err = d.db.QueryRow(`SELECT COUNT(*) FROM symbols`).Scan(&symbolCount)
	return
}

// GetTopFiles returns top files by symbol count descending.
func (d *Database) GetTopFiles(limit int) ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rows, err := d.db.Query(`SELECT file_path FROM symbols GROUP BY file_path ORDER BY COUNT(*) DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []string
	for rows.Next() {
		var fp string
		if err := rows.Scan(&fp); err != nil {
			return nil, err
		}
		files = append(files, fp)
	}
	return files, rows.Err()
}

// FileCount pairs a file path with its symbol count.
type FileCount struct {
	Path  string
	Count int
}

// GetTopFilesWithCounts returns the top files by symbol count, descending, with counts.
func (d *Database) GetTopFilesWithCounts(limit int) ([]FileCount, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rows, err := d.db.Query(
		`SELECT file_path, COUNT(*) AS cnt FROM symbols GROUP BY file_path ORDER BY cnt DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []FileCount
	for rows.Next() {
		var fc FileCount
		if err := rows.Scan(&fc.Path, &fc.Count); err != nil {
			return nil, err
		}
		result = append(result, fc)
	}
	return result, rows.Err()
}


// GetCalleeSymbols returns resolved symbols and unresolved callee names for callerID.
// Resolved: callees whose name matches a symbol in the index.
// Unresolved: callee names with no matching symbol (external/stdlib).
func (d *Database) GetCalleeSymbols(callerID int64) (resolved []Symbol, unresolved []string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rows, err := d.db.Query(`
		SELECT DISTINCT s.`+symbolCols+`
		FROM call_graph cg
		JOIN symbols s ON s.name = cg.callee_name
		WHERE cg.caller_id = ?
		ORDER BY s.file_path, s.start_line`, callerID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	resolved, err = collectSymbols(rows)
	if err != nil {
		return nil, nil, err
	}

	resolvedNames := make(map[string]bool, len(resolved))
	for _, s := range resolved {
		resolvedNames[s.Name] = true
	}

	nameRows, err := d.db.Query(
		`SELECT callee_name FROM call_graph WHERE caller_id = ? ORDER BY callee_name`, callerID)
	if err != nil {
		return nil, nil, err
	}
	defer nameRows.Close()
	for nameRows.Next() {
		var n string
		if err := nameRows.Scan(&n); err != nil {
			return nil, nil, err
		}
		if !resolvedNames[n] {
			unresolved = append(unresolved, n)
		}
	}
	return resolved, unresolved, nameRows.Err()
}

// GetCallersByName returns all symbols that call the given function name.
func (d *Database) GetCallersByName(calleeName string) ([]Symbol, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rows, err := d.db.Query(`
		SELECT s.`+symbolCols+`
		FROM call_graph cg
		JOIN symbols s ON cg.caller_id = s.id
		WHERE cg.callee_name = ?
		ORDER BY s.file_path, s.start_line`, calleeName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSymbols(rows)
}

// FTSSearch performs FTS5 BM25-ranked search.
func (d *Database) FTSSearch(query string, limit int) ([]Symbol, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	ftsQuery := buildFTSQuery(query)
	rows, err := d.db.Query(`
		SELECT s.`+symbolCols+`
		FROM symbols_fts fts
		JOIN symbols s ON s.id = fts.rowid
		WHERE symbols_fts MATCH ?
		ORDER BY rank
		LIMIT ?`, ftsQuery, limit)
	if err != nil {
		return d.keywordSearchLocked(query, limit)
	}
	defer rows.Close()
	syms, err := collectSymbols(rows)
	if err != nil {
		return d.keywordSearchLocked(query, limit)
	}
	return syms, nil
}

func buildFTSQuery(query string) string {
	upper := strings.ToUpper(query)
	if strings.Contains(query, `"`) ||
		strings.Contains(upper, " AND ") ||
		strings.Contains(upper, " OR ") ||
		strings.Contains(upper, " NOT ") {
		return query
	}
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return query
	}
	parts := make([]string, 0, len(terms))
	for _, t := range terms {
		t = strings.ReplaceAll(t, `"`, `""`)
		parts = append(parts, fmt.Sprintf(`name:"%s"* OR signature:"%s"*`, t, t))
	}
	return strings.Join(parts, " OR ")
}

// KeywordSearch uses LIKE '%query%' across name, signature, docstring.
func (d *Database) KeywordSearch(query string, limit int) ([]Symbol, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.keywordSearchLocked(query, limit)
}

// escapeLike escapes LIKE special characters so user input is matched literally.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func (d *Database) keywordSearchLocked(query string, limit int) ([]Symbol, error) {
	pattern := "%" + escapeLike(query) + "%"
	rows, err := d.db.Query(`SELECT `+symbolCols+` FROM symbols
		WHERE name LIKE ? ESCAPE '\' OR signature LIKE ? ESCAPE '\' OR docstring LIKE ? ESCAPE '\'
		LIMIT ?`, pattern, pattern, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSymbols(rows)
}

// FilteredSearch searches symbols by query and optional kind/language/filePattern filters.
// When all filters are empty and a query is set, delegates to FTS with keyword fallback.
// filePattern uses SQLite GLOB syntax (e.g. "*/handlers/*.go").
func (d *Database) FilteredSearch(query, kind, language, filePattern string, limit int) ([]Symbol, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	hasFilters := kind != "" || language != "" || filePattern != ""

	// FTS path (with or without filters).
	if query != "" {
		ftsQuery := buildFTSQuery(query)
		var (
			rows *sql.Rows
			err  error
		)
		if hasFilters {
			rows, err = d.db.Query(`
				SELECT s.`+symbolCols+`
				FROM symbols_fts fts
				JOIN symbols s ON s.id = fts.rowid
				WHERE symbols_fts MATCH ?
				AND (? = '' OR s.kind = ?)
				AND (? = '' OR s.language = ?)
				AND (? = '' OR s.file_path GLOB ?)
				ORDER BY rank
				LIMIT ?`,
				ftsQuery,
				kind, kind,
				language, language,
				filePattern, filePattern,
				limit)
		} else {
			rows, err = d.db.Query(`
				SELECT s.`+symbolCols+`
				FROM symbols_fts fts
				JOIN symbols s ON s.id = fts.rowid
				WHERE symbols_fts MATCH ?
				ORDER BY rank
				LIMIT ?`, ftsQuery, limit)
		}
		if err == nil {
			defer rows.Close()
			syms, serr := collectSymbols(rows)
			if serr == nil && len(syms) > 0 {
				return syms, nil
			}
		}
	}

	// Keyword LIKE fallback with optional filters.
	var clauses []string
	var qargs []interface{}

	if query != "" {
		pattern := "%" + escapeLike(query) + "%"
		clauses = append(clauses, `(name LIKE ? ESCAPE '\' OR signature LIKE ? ESCAPE '\')`)
		qargs = append(qargs, pattern, pattern)
	}
	if kind != "" {
		clauses = append(clauses, `kind = ?`)
		qargs = append(qargs, kind)
	}
	if language != "" {
		clauses = append(clauses, `language = ?`)
		qargs = append(qargs, language)
	}
	if filePattern != "" {
		clauses = append(clauses, `file_path GLOB ?`)
		qargs = append(qargs, filePattern)
	}

	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	qargs = append(qargs, limit)

	rows, err := d.db.Query(`SELECT `+symbolCols+` FROM symbols`+where+` LIMIT ?`, qargs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSymbols(rows)
}

// ListIndexedFiles returns all indexed file paths in alphabetical order.
func (d *Database) ListIndexedFiles() ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rows, err := d.db.Query(`SELECT file_path FROM indexed_files ORDER BY file_path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var fp string
		if err := rows.Scan(&fp); err != nil {
			return nil, err
		}
		paths = append(paths, fp)
	}
	return paths, rows.Err()
}
