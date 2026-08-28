package watcher

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AvengeMedia/danksearch/internal/config"
	mocks_watcher "github.com/AvengeMedia/danksearch/internal/mocks/watcher"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type countingIndexer struct {
	*mocks_watcher.MockIndexer
	indexed atomic.Int64
	deleted atomic.Int64
}

func newCountingIndexer(t *testing.T) *countingIndexer {
	ci := &countingIndexer{MockIndexer: mocks_watcher.NewMockIndexer(t)}
	ci.EXPECT().Index(mock.Anything).RunAndReturn(func(string) error {
		ci.indexed.Add(1)
		return nil
	}).Maybe()
	ci.EXPECT().Delete(mock.Anything).RunAndReturn(func(string) error {
		ci.deleted.Add(1)
		return nil
	}).Maybe()
	return ci
}

type WatcherSuite struct {
	suite.Suite
}

func TestWatcherSuite(t *testing.T) {
	suite.Run(t, new(WatcherSuite))
}

func (s *WatcherSuite) TestNew() {
	cfg := config.Default()
	idx := newCountingIndexer(s.T())

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

	w, err := New(newCountingIndexer(s.T()), cfg)
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
	idx := newCountingIndexer(s.T())

	w, err := New(idx, cfg)
	s.Require().NoError(err)
	s.Require().NoError(w.Start())
	defer w.Stop()

	testFile := filepath.Join(tmpDir, "test.txt")
	s.Require().NoError(os.WriteFile(testFile, []byte("hello"), 0644))
	time.Sleep(100 * time.Millisecond)
	s.NotZero(idx.indexed.Load())

	s.Require().NoError(os.WriteFile(testFile, []byte("world"), 0644))
	time.Sleep(100 * time.Millisecond)
	s.GreaterOrEqual(idx.indexed.Load(), int64(2))

	s.Require().NoError(os.Remove(testFile))
	time.Sleep(100 * time.Millisecond)
	s.NotZero(idx.deleted.Load())
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
	idx := newCountingIndexer(s.T())

	excludedDir := filepath.Join(tmpDir, ".git")
	s.Require().NoError(os.Mkdir(excludedDir, 0755))

	w, err := New(idx, cfg)
	s.Require().NoError(err)
	s.Require().NoError(w.Start())
	defer w.Stop()

	testFile := filepath.Join(excludedDir, "config.txt")
	s.Require().NoError(os.WriteFile(testFile, []byte("test"), 0644))
	time.Sleep(100 * time.Millisecond)

	s.Zero(idx.indexed.Load())
}
