// Package watcher provides file-system watching with debounced reindexing.
package watcher

import (
	"io/fs"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"gocode-outline-graph/internal/db"
	"gocode-outline-graph/internal/indexer"
	"gocode-outline-graph/internal/parser"
)

const debounceDuration = 500 * time.Millisecond

// CodeWatcher watches a project directory and reindexes files on change.
type CodeWatcher struct {
	indexer  *indexer.Indexer
	database *db.Database
	watcher  *fsnotify.Watcher
	done     chan struct{}
	debounce map[string]*time.Timer
	mu       sync.Mutex
}

// New creates a CodeWatcher. Call Start to begin watching.
func New(idx *indexer.Indexer, database *db.Database) *CodeWatcher {
	return &CodeWatcher{
		indexer:  idx,
		database: database,
		done:     make(chan struct{}),
		debounce: make(map[string]*time.Timer),
	}
}

// Start begins watching projectPath and all its subdirectories recursively.
// It returns an error if the underlying watcher cannot be created or if
// the root directory cannot be added.
func (w *CodeWatcher) Start(projectPath string) error {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	w.watcher = fw

	// Add the root directory.
	if err = fw.Add(projectPath); err != nil {
		fw.Close()
		return err
	}

	// Walk subdirectories and add each one.
	_ = filepath.WalkDir(projectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && path != projectPath {
			// Best-effort: ignore errors for individual directories.
			_ = fw.Add(path)
		}
		return nil
	})

	go w.eventLoop()
	return nil
}

// Stop halts the watcher and its background goroutine.
func (w *CodeWatcher) Stop() {
	close(w.done)
	if w.watcher != nil {
		w.watcher.Close()
	}

	// Cancel any pending debounce timers.
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, t := range w.debounce {
		t.Stop()
	}
}

// eventLoop reads fsnotify events and debounces per-file reindexing.
func (w *CodeWatcher) eventLoop() {
	for {
		select {
		case <-w.done:
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("watcher: error: %v", err)
		}
	}
}

// handleEvent debounces the event by 500 ms and then dispatches to the
// appropriate handler based on the operation.
func (w *CodeWatcher) handleEvent(event fsnotify.Event) {
	path := event.Name
	op := event.Op

	// Only process files we can parse.
	if !parser.IsSupported(path) {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Cancel any existing pending timer for this path.
	if t, exists := w.debounce[path]; exists {
		t.Stop()
	}

	w.debounce[path] = time.AfterFunc(debounceDuration, func() {
		w.mu.Lock()
		delete(w.debounce, path)
		w.mu.Unlock()

		w.dispatch(path, op)
	})
}

// dispatch performs the actual indexing or removal action after the debounce
// period has elapsed.
func (w *CodeWatcher) dispatch(path string, op fsnotify.Op) {
	switch {
	case op&(fsnotify.Create|fsnotify.Write) != 0:
		if err := w.indexer.IndexFile(path); err != nil {
			log.Printf("watcher: reindex %q: %v", path, err)
		}
	case op&(fsnotify.Remove|fsnotify.Rename) != 0:
		if err := w.database.DeleteSymbolsForFile(path); err != nil {
			log.Printf("watcher: delete symbols for %q: %v", path, err)
		}
	}
}
