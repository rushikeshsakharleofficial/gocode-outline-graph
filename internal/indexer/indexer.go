// Package indexer orchestrates full-project and single-file indexing.
package indexer

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"gocode-outline-graph/internal/db"
	"gocode-outline-graph/internal/parser"
)

// maxFileSize is the default upper limit for files to index (512 KB).
const maxFileSize = 512 * 1024

// secretFiles is the set of basenames that are always skipped.
var secretFiles = map[string]struct{}{
	".env":            {},
	".env.local":      {},
	".env.production": {},
	".env.development": {},
}

// skipDirs is the set of directory names that are always skipped.
var skipDirs = map[string]struct{}{
	"node_modules":  {},
	"__pycache__":   {},
	".git":          {},
	"dist":          {},
	"build":         {},
	".venv":         {},
	"venv":          {},
	"vendor":        {},
	"target":        {},
	".next":         {},
	".nuxt":         {},
	"coverage":      {},
	"__snapshots__": {},
}

// gitignorePattern represents a single parsed .gitignore rule.
type gitignorePattern struct {
	raw      string
	negated  bool
	dirOnly  bool
	anchored bool   // pattern contains a slash before the last segment
	segments []string
}

// gitignoreMatcher holds compiled patterns for a single .gitignore file.
type gitignoreMatcher struct {
	root     string
	patterns []gitignorePattern
}

// parseGitignoreFile reads a .gitignore file and returns a matcher anchored at dir.
func parseGitignoreFile(gitignorePath string) (*gitignoreMatcher, error) {
	f, err := os.Open(gitignorePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dir := filepath.Dir(gitignorePath)
	m := &gitignoreMatcher{root: dir}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Strip trailing spaces (not escaped).
		line = strings.TrimRight(line, " \t\r")

		// Skip blank lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		pat := gitignorePattern{raw: line}

		// Handle negation.
		if strings.HasPrefix(line, "!") {
			pat.negated = true
			line = line[1:]
		}

		// Handle dir-only (trailing slash).
		if strings.HasSuffix(line, "/") {
			pat.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}

		// A pattern is "anchored" (only matched relative to the .gitignore root)
		// if it contains a slash anywhere except as the very first character.
		stripped := strings.TrimPrefix(line, "/")
		if strings.Contains(stripped, "/") {
			pat.anchored = true
		}
		// Remove a leading slash if present.
		line = strings.TrimPrefix(line, "/")

		pat.segments = strings.Split(line, "/")
		m.patterns = append(m.patterns, pat)
	}
	return m, scanner.Err()
}

// matchesSegment returns true if name matches a single glob pattern segment.
func matchesSegment(pattern, name string) bool {
	matched, err := filepath.Match(pattern, name)
	return err == nil && matched
}

// matchPath checks whether relPath (relative to m.root, using forward slashes)
// matches any pattern in the matcher. It returns (ignored bool).
func (m *gitignoreMatcher) matchPath(relPath string, isDir bool) bool {
	ignored := false
	parts := strings.Split(relPath, string(filepath.Separator))

	for _, pat := range m.patterns {
		var matched bool

		if pat.dirOnly && !isDir {
			continue
		}

		if pat.anchored {
			// Must match the full relative path from the root.
			matched = matchSegments(pat.segments, parts)
		} else {
			// Try matching against every suffix of parts.
			for start := 0; start <= len(parts)-len(pat.segments); start++ {
				if matchSegments(pat.segments, parts[start:]) {
					matched = true
					break
				}
			}
		}

		if matched {
			if pat.negated {
				ignored = false
			} else {
				ignored = true
			}
		}
	}
	return ignored
}

// matchSegments returns true if segs matches the leading portion of parts.
// It handles ** by matching zero or more path components.
func matchSegments(segs, parts []string) bool {
	si, pi := 0, 0
	for si < len(segs) && pi < len(parts) {
		if segs[si] == "**" {
			// ** matches zero or more path components.
			si++
			if si == len(segs) {
				return true // ** at end matches everything remaining
			}
			// Try matching the rest starting at every position.
			for ; pi <= len(parts); pi++ {
				if matchSegments(segs[si:], parts[pi:]) {
					return true
				}
			}
			return false
		}
		if !matchesSegment(segs[si], parts[pi]) {
			return false
		}
		si++
		pi++
	}
	// Consume trailing ** segments.
	for si < len(segs) && segs[si] == "**" {
		si++
	}
	return si == len(segs) && pi == len(parts)
}

// Indexer orchestrates project and single-file indexing.
type Indexer struct {
	database   *db.Database
	workers    int
	OnProgress func(done, total int, filePath string) // called after each file; nil = no-op; must be goroutine-safe
}

// New creates a new Indexer. If workers <= 0, it defaults to min(cpu_count, 4).
func New(database *db.Database, workers int) *Indexer {
	if workers <= 0 {
		workers = runtime.NumCPU()
		if workers > 4 {
			workers = 4
		}
	}
	if envVal := os.Getenv("CODE_OUTLINE_INDEX_WORKERS"); envVal != "" {
		if n, err := strconv.Atoi(envVal); err == nil && n > 0 {
			workers = n
		}
	}
	return &Indexer{database: database, workers: workers}
}

// checksumBytes computes a SHA-256 hex checksum of data.
func checksumBytes(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// IndexAll indexes all supported files under projectPath using a worker pool.
// Returns the number of files successfully indexed (written to the DB).
func (idx *Indexer) IndexAll(projectPath string) (int, error) {
	allowLarge := os.Getenv("CODE_OUTLINE_BACKGROUND_LARGE_FILES") == "1"

	// Collect .gitignore matchers found during the walk.
	// We use a slice; it's populated before we process files.
	var matchers []*gitignoreMatcher
	var matchersMu sync.Mutex

	// First pass: collect files to index and .gitignore matchers.
	var files []string

	err := filepath.WalkDir(projectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}

		name := d.Name()

		if d.IsDir() {
			// Always skip dotfile dirs (except the project root itself).
			if path != projectPath && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			if _, skip := skipDirs[name]; skip {
				return filepath.SkipDir
			}

			// Load .gitignore from this directory if present.
			giPath := filepath.Join(path, ".gitignore")
			if m, err2 := parseGitignoreFile(giPath); err2 == nil {
				matchersMu.Lock()
				matchers = append(matchers, m)
				matchersMu.Unlock()
			}
			return nil
		}

		// Check secret filenames.
		if _, secret := secretFiles[name]; secret {
			return nil
		}

		// Check parser support.
		if !parser.IsSupported(path) {
			return nil
		}

		// File size check.
		info, err2 := d.Info()
		if err2 != nil {
			return nil
		}
		if !allowLarge && info.Size() >= maxFileSize {
			return nil
		}

		// Gitignore check: test against every matcher whose root is an
		// ancestor of path.
		matchersMu.Lock()
		ignored := false
		for _, m := range matchers {
			rel, err3 := filepath.Rel(m.root, path)
			if err3 != nil || strings.HasPrefix(rel, "..") {
				continue
			}
			if m.matchPath(rel, false) {
				ignored = true
			}
		}
		matchersMu.Unlock()
		if ignored {
			return nil
		}

		files = append(files, path)
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("indexer: walk error: %w", err)
	}

	if len(files) == 0 {
		return 0, nil
	}

	total := len(files)
	jobs := make(chan string, total)
	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	var indexed int64
	var wg sync.WaitGroup

	for i := 0; i < idx.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := parser.New()
			for filePath := range jobs {
				if err2 := idx.indexFileWithParser(filePath, p); err2 == nil {
					n := int(atomic.AddInt64(&indexed, 1))
					if idx.OnProgress != nil {
						idx.OnProgress(n, total, filePath)
					}
				}
			}
		}()
	}
	wg.Wait()

	return int(atomic.LoadInt64(&indexed)), nil
}

// IndexFile (re)indexes a single file, creating its own parser instance.
func (idx *Indexer) IndexFile(filePath string) error {
	if !parser.IsSupported(filePath) {
		return nil
	}
	p := parser.New()
	return idx.indexFileWithParser(filePath, p)
}

// indexFileWithParser performs the actual read → checksum → parse → insert sequence.
func (idx *Indexer) indexFileWithParser(filePath string, p *parser.SymbolParser) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	size := info.Size()
	mtimeNs := info.ModTime().UnixNano()

	// Fast-path: skip if file hasn't changed.
	if idx.database.IsFileCurrent(filePath, size, mtimeNs) {
		return nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	checksum := checksumBytes(data)

	// Double-check with checksum in case mtime was bumped without content change.
	existing := idx.database.GetChecksumForFile(filePath)
	if existing == checksum {
		return nil
	}

	lang := parser.DetectLanguage(filePath)
	symbols := p.Parse(filePath, data, lang)

	return idx.database.InsertSymbolsForFile(filePath, symbols, checksum, mtimeNs, size)
}

// EnsureFresh checks if the file is current in the DB; if not, reindexes it.
func (idx *Indexer) EnsureFresh(filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		// File may have been deleted; leave DB cleanup to RemoveStale.
		return nil
	}
	if idx.database.IsFileCurrent(filePath, info.Size(), info.ModTime().UnixNano()) {
		return nil
	}
	return idx.IndexFile(filePath)
}

// RemoveStale removes DB entries for files that no longer exist on disk.
// It lists all indexed files under projectPath and deletes those that are missing.
// Returns the number of removed entries.
func (idx *Indexer) RemoveStale(projectPath string) (int, error) {
	all, err := idx.database.ListIndexedFiles()
	if err != nil {
		return 0, fmt.Errorf("indexer: list indexed files: %w", err)
	}

	absProject, err := filepath.Abs(projectPath)
	if err != nil {
		return 0, err
	}

	removed := 0
	for _, fp := range all {
		// Only consider files that belong to this project.
		absFP, err2 := filepath.Abs(fp)
		if err2 != nil {
			continue
		}
		if !strings.HasPrefix(absFP, absProject+string(filepath.Separator)) &&
			absFP != absProject {
			continue
		}

		if _, err2 = os.Stat(fp); os.IsNotExist(err2) {
			if err3 := idx.database.DeleteFile(fp); err3 == nil {
				removed++
			}
		}
	}
	return removed, nil
}

