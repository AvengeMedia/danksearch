package watcher

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/AvengeMedia/danksearch/internal/config"
	"github.com/stretchr/testify/suite"
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

type WatcherSuite struct {
	suite.Suite
}

func TestWatcherSuite(t *testing.T) {
	suite.Run(t, new(WatcherSuite))
}

func (s *WatcherSuite) TestNew() {
	cfg := config.Default()
	idx := &mockIndexer{}

	w, err := New(idx, cfg)
	s.Require().NoError(err)
	s.NotNil(w.watcher)
	s.Equal(idx, w.indexer)
	s.Equal(cfg, w.config)
}

func (s *WatcherSuite) TestStartStop() {
	tmpDir := s.T().TempDir()
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

	w, err := New(&mockIndexer{}, cfg)
	s.Require().NoError(err)

	s.False(w.IsRunning())

	s.Require().NoError(w.Start())
	s.True(w.IsRunning())

	s.NoError(w.Start(), "Start() should be idempotent")

	s.NoError(w.Stop())
	s.False(w.IsRunning())
}

func (s *WatcherSuite) TestFileEvents() {
	tmpDir := s.T().TempDir()
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
	s.Require().NoError(err)
	s.Require().NoError(w.Start())
	defer w.Stop()

	testFile := filepath.Join(tmpDir, "test.txt")
	s.Require().NoError(os.WriteFile(testFile, []byte("hello"), 0644))
	time.Sleep(100 * time.Millisecond)
	s.NotZero(idx.indexedCount())

	s.Require().NoError(os.WriteFile(testFile, []byte("world"), 0644))
	time.Sleep(100 * time.Millisecond)
	s.GreaterOrEqual(idx.indexedCount(), 2)

	s.Require().NoError(os.Remove(testFile))
	time.Sleep(100 * time.Millisecond)
	s.NotZero(idx.deletedCount())
}

func (s *WatcherSuite) TestExcludedDirs() {
	tmpDir := s.T().TempDir()
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
	s.Require().NoError(os.Mkdir(excludedDir, 0755))

	w, err := New(idx, cfg)
	s.Require().NoError(err)
	s.Require().NoError(w.Start())
	defer w.Stop()

	testFile := filepath.Join(excludedDir, "config.txt")
	s.Require().NoError(os.WriteFile(testFile, []byte("test"), 0644))
	time.Sleep(100 * time.Millisecond)

	s.Zero(idx.indexedCount())
}
