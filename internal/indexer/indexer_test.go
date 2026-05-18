package indexer_test

import (
	"os"
	"path/filepath"
	"testing"

	"gocode-outline-graph/internal/db"
	"gocode-outline-graph/internal/indexer"
)

func openTestDB(t *testing.T) *db.Database {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "index.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

// TestForce_reindexesDespiteFreshnessMatch verifies that Force=true causes
// re-indexing even when the file appears current (mtime + size unchanged).
//
// Strategy: index a file, then overwrite it with same-length content and
// restore the original mtime via os.Chtimes so IsFileCurrent returns true.
// Without Force the new symbol is invisible; with Force it appears.
func TestForce_reindexesDespiteFreshnessMatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "pkg.go")

	// v1 and v2 must have identical byte lengths so size check passes.
	v1 := []byte("package pkg\n\nfunc Hello() {}\n")
	v2 := []byte("package pkg\n\nfunc World() {}\n")
	if len(v1) != len(v2) {
		t.Fatal("test setup: v1 and v2 must have identical byte lengths")
	}

	if err := os.WriteFile(filePath, v1, 0644); err != nil {
		t.Fatal(err)
	}

	database := openTestDB(t)
	idx := indexer.New(database, 1)

	// First index: Hello should appear.
	if _, err := idx.IndexAll(dir); err != nil {
		t.Fatalf("IndexAll v1: %v", err)
	}
	syms, _ := database.GetSymbolsForFile(filePath)
	if len(syms) == 0 || syms[0].Name != "Hello" {
		t.Fatalf("expected Hello after v1 index, got %v", syms)
	}

	// Capture original mtime.
	fi, err := os.Stat(filePath)
	if err != nil {
		t.Fatal(err)
	}
	origMtime := fi.ModTime()

	// Overwrite with v2 (same size), then restore original mtime.
	if err := os.WriteFile(filePath, v2, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filePath, origMtime, origMtime); err != nil {
		t.Fatal(err)
	}

	// Without Force: IsFileCurrent returns true → Hello still in DB.
	idx2 := indexer.New(database, 1)
	idx2.Force = false
	if _, err := idx2.IndexAll(dir); err != nil {
		t.Fatalf("IndexAll no-force: %v", err)
	}
	syms, _ = database.GetSymbolsForFile(filePath)
	if len(syms) == 0 || syms[0].Name != "Hello" {
		t.Fatalf("expected Hello still present (no force), got %v", syms)
	}

	// With Force: bypasses IsFileCurrent → World should appear.
	idx3 := indexer.New(database, 1)
	idx3.Force = true
	if _, err := idx3.IndexAll(dir); err != nil {
		t.Fatalf("IndexAll force: %v", err)
	}
	syms, _ = database.GetSymbolsForFile(filePath)
	found := false
	for _, s := range syms {
		if s.Name == "World" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected World after force re-index, got %v", syms)
	}
}
