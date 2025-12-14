package watcher

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/AvengeMedia/danksearch/internal/config"
)

type mockIndexer struct {
	indexed []string
	deleted []string
	mu      sync.Mutex
}

func (m *mockIndexer) Index(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.indexed = append(m.indexed, path)
	return nil
}

func (m *mockIndexer) Delete(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleted = append(m.deleted, path)
	return nil
}

func (m *mockIndexer) indexedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.indexed)
}

func (m *mockIndexer) deletedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.deleted)
}

func TestNew(t *testing.T) {
	cfg := config.Default()
	idx := &mockIndexer{}

	w, err := New(idx, cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if w.watcher == nil {
		t.Error("watcher should not be nil")
	}

	if w.indexer != idx {
		t.Error("indexer should match")
	}

	if w.config != cfg {
		t.Error("config should match")
	}
}

func TestWatcher_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Default()
	cfg.IndexPaths = []config.IndexPath{
		{
			Path:          tmpDir,
			MaxDepth:      10,
			ExcludeHidden: false,
			ExcludeDirs:   []string{},
		},
	}
	cfg.BuildMaps()
	idx := &mockIndexer{}

	w, err := New(idx, cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if w.IsRunning() {
		t.Error("watcher should not be running initially")
	}

	if err := w.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !w.IsRunning() {
		t.Error("watcher should be running after Start()")
	}

	if err := w.Start(); err != nil {
		t.Error("Start() should be idempotent")
	}

	if err := w.Stop(); err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	if w.IsRunning() {
		t.Error("watcher should not be running after Stop()")
	}
}

func TestWatcher_FileEvents(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Default()
	cfg.IndexPaths = []config.IndexPath{
		{
			Path:          tmpDir,
			MaxDepth:      10,
			ExcludeHidden: false,
			ExcludeDirs:   []string{},
		},
	}
	cfg.BuildMaps()
	idx := &mockIndexer{}

	w, err := New(idx, cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := w.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer w.Stop()

	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if idx.indexedCount() == 0 {
		t.Error("expected file to be indexed")
	}

	if err := os.WriteFile(testFile, []byte("world"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if idx.indexedCount() < 2 {
		t.Error("expected file to be reindexed after modification")
	}

	if err := os.Remove(testFile); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if idx.deletedCount() == 0 {
		t.Error("expected file to be deleted from index")
	}
}

func TestWatcher_ExcludedDirs(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Default()
	cfg.IndexPaths = []config.IndexPath{
		{
			Path:          tmpDir,
			MaxDepth:      10,
			ExcludeHidden: false,
			ExcludeDirs:   []string{".git"},
		},
	}
	cfg.BuildMaps()
	idx := &mockIndexer{}

	excludedDir := filepath.Join(tmpDir, ".git")
	if err := os.Mkdir(excludedDir, 0755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	w, err := New(idx, cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := w.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer w.Stop()

	testFile := filepath.Join(excludedDir, "config.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if idx.indexedCount() > 0 {
		t.Error("files in excluded directories should not be indexed")
	}
}
