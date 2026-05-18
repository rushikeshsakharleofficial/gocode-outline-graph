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

func TestFilteredSearch_byKind(t *testing.T) {
	d := openTestDB(t)
	insertSymbols(t, d, "main.go", []db.Symbol{
		{Name: "RunServer", Kind: "function", Language: "go", StartLine: 1, EndLine: 5},
		{Name: "Config", Kind: "struct", Language: "go", StartLine: 10, EndLine: 20},
	})
	results, err := d.FilteredSearch("", "function", "", "", 20)
	if err != nil {
		t.Fatalf("FilteredSearch: %v", err)
	}
	if len(results) != 1 || results[0].Name != "RunServer" {
		t.Errorf("want [RunServer], got %v", results)
	}
}

func TestFilteredSearch_byLanguage(t *testing.T) {
	d := openTestDB(t)
	insertSymbols(t, d, "main.go", []db.Symbol{
		{Name: "GoFunc", Kind: "function", Language: "go"},
	})
	insertSymbols(t, d, "app.py", []db.Symbol{
		{Name: "py_func", Kind: "function", Language: "python"},
	})
	results, err := d.FilteredSearch("", "", "python", "", 20)
	if err != nil {
		t.Fatalf("FilteredSearch: %v", err)
	}
	if len(results) != 1 || results[0].Name != "py_func" {
		t.Errorf("want [py_func], got %v", results)
	}
}

func TestFilteredSearch_byFilePattern(t *testing.T) {
	d := openTestDB(t)
	insertSymbols(t, d, "/project/handlers/auth.go", []db.Symbol{
		{Name: "Login", Kind: "function", Language: "go"},
	})
	insertSymbols(t, d, "/project/models/user.go", []db.Symbol{
		{Name: "User", Kind: "struct", Language: "go"},
	})
	results, err := d.FilteredSearch("", "", "", "*/handlers/*", 20)
	if err != nil {
		t.Fatalf("FilteredSearch: %v", err)
	}
	if len(results) != 1 || results[0].Name != "Login" {
		t.Errorf("want [Login], got %v", results)
	}
}

func TestFilteredSearch_queryWithKind(t *testing.T) {
	d := openTestDB(t)
	insertSymbols(t, d, "main.go", []db.Symbol{
		{Name: "HandleRequest", Kind: "function", Language: "go", StartLine: 1, EndLine: 5,
			Signature: "func HandleRequest(w http.ResponseWriter)"},
		{Name: "RequestConfig", Kind: "struct", Language: "go", StartLine: 10, EndLine: 15},
	})
	// Query matches both, but kind filter restricts to function only.
	results, err := d.FilteredSearch("Request", "function", "", "", 20)
	if err != nil {
		t.Fatalf("FilteredSearch: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("want at least one result for 'Request' + kind=function, got none")
	}
	for _, s := range results {
		if s.Kind != "function" {
			t.Errorf("unexpected kind %q for %q", s.Kind, s.Name)
		}
	}
}

func TestGetSymbolsByName_multipleFiles(t *testing.T) {
	d := openTestDB(t)
	insertSymbols(t, d, "pkg/a/main.go", []db.Symbol{
		{Name: "Handler", Kind: "function", Language: "go", StartLine: 1, EndLine: 5},
	})
	insertSymbols(t, d, "pkg/b/main.go", []db.Symbol{
		{Name: "Handler", Kind: "function", Language: "go", StartLine: 10, EndLine: 15},
	})

	all, err := d.GetSymbolsByName("Handler", "")
	if err != nil {
		t.Fatalf("GetSymbolsByName: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("want 2 results, got %d", len(all))
	}

	one, err := d.GetSymbolsByName("Handler", "pkg/a/main.go")
	if err != nil {
		t.Fatalf("GetSymbolsByName with file: %v", err)
	}
	if len(one) != 1 || one[0].FilePath != "pkg/a/main.go" {
		t.Errorf("want pkg/a/main.go symbol, got %v", one)
	}
}

func TestGetSymbolsByKind(t *testing.T) {
	d := openTestDB(t)
	insertSymbols(t, d, "main.go", []db.Symbol{
		{Name: "RunServer", Kind: "function", Language: "go", StartLine: 1, EndLine: 5},
		{Name: "Config", Kind: "struct", Language: "go", StartLine: 10, EndLine: 20},
		{Name: "NewConfig", Kind: "function", Language: "go", StartLine: 25, EndLine: 30},
	})

	fns, err := d.GetSymbolsByKind("function", "", 200)
	if err != nil {
		t.Fatalf("GetSymbolsByKind: %v", err)
	}
	if len(fns) != 2 {
		t.Errorf("want 2 functions, got %d", len(fns))
	}

	structs, err := d.GetSymbolsByKind("struct", "", 200)
	if err != nil {
		t.Fatalf("GetSymbolsByKind struct: %v", err)
	}
	if len(structs) != 1 || structs[0].Name != "Config" {
		t.Errorf("want [Config], got %v", structs)
	}
}

func TestGetSymbolsByKind_withLanguageFilter(t *testing.T) {
	d := openTestDB(t)
	insertSymbols(t, d, "main.go", []db.Symbol{
		{Name: "GoFunc", Kind: "function", Language: "go"},
	})
	insertSymbols(t, d, "app.py", []db.Symbol{
		{Name: "py_func", Kind: "function", Language: "python"},
	})

	results, err := d.GetSymbolsByKind("function", "go", 200)
	if err != nil {
		t.Fatalf("GetSymbolsByKind: %v", err)
	}
	if len(results) != 1 || results[0].Name != "GoFunc" {
		t.Errorf("want [GoFunc], got %v", results)
	}
}
