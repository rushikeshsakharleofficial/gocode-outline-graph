package db_test

import (
	"path/filepath"
	"testing"
	"time"

	"gocode-outline-graph/internal/db"
)

// openTestDB creates a temporary SQLite DB for the duration of t.
func openTestDB(t *testing.T) *db.Database {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

// insertSymbols inserts symbols into the DB for filePath, stamping each symbol's FilePath.
func insertSymbols(t *testing.T, d *db.Database, filePath string, syms []db.Symbol) {
	t.Helper()
	for i := range syms {
		syms[i].FilePath = filePath
	}
	if err := d.InsertSymbolsForFile(filePath, syms, "checksum-"+filePath, time.Now().UnixNano(), 100); err != nil {
		t.Fatalf("insert symbols for %q: %v", filePath, err)
	}
}

func TestGetCalleeSymbols(t *testing.T) {
	d := openTestDB(t)

	insertSymbols(t, d, "caller.go", []db.Symbol{
		{Name: "MyFunc", Kind: "function", Language: "go", StartLine: 1, EndLine: 5,
			Calls: []string{"HelperA", "externalPkg.Thing"}},
	})
	insertSymbols(t, d, "helper.go", []db.Symbol{
		{Name: "HelperA", Kind: "function", Language: "go", StartLine: 1, EndLine: 3},
	})

	caller, err := d.GetSymbolByName("MyFunc", "caller.go")
	if err != nil || caller == nil {
		t.Fatalf("get caller: %v", err)
	}

	resolved, unresolved, err := d.GetCalleeSymbols(caller.ID)
	if err != nil {
		t.Fatalf("GetCalleeSymbols: %v", err)
	}

	if len(resolved) != 1 || resolved[0].Name != "HelperA" {
		t.Errorf("want 1 resolved (HelperA), got %v", resolved)
	}
	if len(unresolved) != 1 || unresolved[0] != "externalPkg.Thing" {
		t.Errorf("want 1 unresolved (externalPkg.Thing), got %v", unresolved)
	}
}

func TestGetTopFilesWithCounts(t *testing.T) {
	d := openTestDB(t)

	insertSymbols(t, d, "a.go", []db.Symbol{
		{Name: "Foo", Kind: "function", Language: "go"},
		{Name: "Bar", Kind: "function", Language: "go"},
		{Name: "Baz", Kind: "function", Language: "go"},
	})
	insertSymbols(t, d, "b.go", []db.Symbol{
		{Name: "Qux", Kind: "function", Language: "go"},
	})

	results, err := d.GetTopFilesWithCounts(10)
	if err != nil {
		t.Fatalf("GetTopFilesWithCounts: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	if results[0].Path != "a.go" {
		t.Errorf("want first=a.go, got %q", results[0].Path)
	}
	if results[0].Count != 3 {
		t.Errorf("want count=3, got %d", results[0].Count)
	}
	if results[1].Path != "b.go" || results[1].Count != 1 {
		t.Errorf("unexpected second result: %+v", results[1])
	}
}
