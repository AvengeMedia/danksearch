package server

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/AvengeMedia/danksearch/internal/config"
	"github.com/AvengeMedia/danksearch/internal/indexer"
	"github.com/AvengeMedia/danksearch/internal/server/models"
	bleve "github.com/blevesearch/bleve/v2"
	"github.com/stretchr/testify/suite"
)

type mockRouterIndexer struct{}

func (m *mockRouterIndexer) Search(query string, limit int) (*bleve.SearchResult, error) {
	return &bleve.SearchResult{Total: 5}, nil
}

func (m *mockRouterIndexer) SearchWithOptions(opts *indexer.SearchOptions) (*bleve.SearchResult, error) {
	return &bleve.SearchResult{Total: 5}, nil
}

func (m *mockRouterIndexer) SearchAll(opts *indexer.SearchOptions) (*indexer.SearchResult, error) {
	return &indexer.SearchResult{SearchResult: &bleve.SearchResult{Total: 5}}, nil
}

func (m *mockRouterIndexer) ReindexAll() error {
	return nil
}

func (m *mockRouterIndexer) SyncIncremental() error {
	return nil
}

func (m *mockRouterIndexer) Stats() *config.IndexStats {
	return &config.IndexStats{TotalFiles: 100}
}

func (m *mockRouterIndexer) ListFiles(prefix string, limit int) ([]indexer.FileEntry, int, error) {
	return nil, 0, nil
}

type mockRouterWatcher struct {
	running bool
}

func (m *mockRouterWatcher) Start() error {
	m.running = true
	return nil
}

func (m *mockRouterWatcher) Stop() error {
	m.running = false
	return nil
}

func (m *mockRouterWatcher) IsRunning() bool {
	return m.running
}

type mockConn struct {
	net.Conn
	written []byte
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	m.written = append(m.written, b...)
	return len(b), nil
}

func (m *mockConn) Close() error {
	return nil
}

type RouterSuite struct {
	suite.Suite
}

func TestRouterSuite(t *testing.T) {
	suite.Run(t, new(RouterSuite))
}

func (s *RouterSuite) TestPing() {
	router := NewRouter(&mockRouterIndexer{}, &mockRouterWatcher{})
	conn := &mockConn{}
	router.RouteRequest(conn, models.Request{ID: 1, Method: "ping"})

	var resp models.Response[string]
	s.Require().NoError(json.Unmarshal(conn.written, &resp))
	s.Equal(1, resp.ID)
	s.Require().NotNil(resp.Result)
	s.Equal("pong", *resp.Result)
}

func (s *RouterSuite) TestSearch() {
	router := NewRouter(&mockRouterIndexer{}, &mockRouterWatcher{})
	conn := &mockConn{}
	router.RouteRequest(conn, models.Request{
		ID:     2,
		Method: "search",
		Params: map[string]any{"query": "test", "limit": float64(10)},
	})

	var resp models.Response[bleve.SearchResult]
	s.Require().NoError(json.Unmarshal(conn.written, &resp))
	s.Equal(2, resp.ID)
	s.Require().NotNil(resp.Result)
	s.Equal(uint64(5), resp.Result.Total)
}

func (s *RouterSuite) TestStats() {
	router := NewRouter(&mockRouterIndexer{}, &mockRouterWatcher{})
	conn := &mockConn{}
	router.RouteRequest(conn, models.Request{ID: 3, Method: "stats"})

	var resp models.Response[config.IndexStats]
	s.Require().NoError(json.Unmarshal(conn.written, &resp))
	s.Require().NotNil(resp.Result)
	s.EqualValues(100, resp.Result.TotalFiles)
}

func (s *RouterSuite) TestWatchStart() {
	w := &mockRouterWatcher{}
	router := NewRouter(&mockRouterIndexer{}, w)
	conn := &mockConn{}
	router.RouteRequest(conn, models.Request{ID: 4, Method: "watch.start"})

	var resp models.Response[map[string]string]
	s.Require().NoError(json.Unmarshal(conn.written, &resp))
	s.Require().NotNil(resp.Result)
	s.Equal("watcher started", (*resp.Result)["status"])
	s.True(w.running)
}

func (s *RouterSuite) TestWatchStop() {
	w := &mockRouterWatcher{running: true}
	router := NewRouter(&mockRouterIndexer{}, w)
	conn := &mockConn{}
	router.RouteRequest(conn, models.Request{ID: 5, Method: "watch.stop"})

	var resp models.Response[map[string]string]
	s.Require().NoError(json.Unmarshal(conn.written, &resp))
	s.Require().NotNil(resp.Result)
	s.Equal("watcher stopped", (*resp.Result)["status"])
	s.False(w.running)
}

func (s *RouterSuite) TestWatchStatus() {
	tests := []struct {
		name     string
		running  bool
		expected string
	}{
		{"running", true, "running"},
		{"stopped", false, "stopped"},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			router := NewRouter(&mockRouterIndexer{}, &mockRouterWatcher{running: tt.running})
			conn := &mockConn{}
			router.RouteRequest(conn, models.Request{ID: 6, Method: "watch.status"})

			var resp models.Response[map[string]string]
			s.Require().NoError(json.Unmarshal(conn.written, &resp))
			s.Require().NotNil(resp.Result)
			s.Equal(tt.expected, (*resp.Result)["status"])
		})
	}
}

func (s *RouterSuite) TestReindex() {
	router := NewRouter(&mockRouterIndexer{}, &mockRouterWatcher{})
	conn := &mockConn{}
	router.RouteRequest(conn, models.Request{ID: 7, Method: "reindex"})
	time.Sleep(50 * time.Millisecond)

	var resp models.Response[map[string]string]
	s.Require().NoError(json.Unmarshal(conn.written, &resp))
	s.Require().NotNil(resp.Result)
	s.Equal("reindexing started", (*resp.Result)["status"])
}

func (s *RouterSuite) TestUnknownMethod() {
	router := NewRouter(&mockRouterIndexer{}, &mockRouterWatcher{})
	conn := &mockConn{}
	router.RouteRequest(conn, models.Request{ID: 8, Method: "unknown"})

	var resp models.Response[any]
	s.Require().NoError(json.Unmarshal(conn.written, &resp))
	s.NotEmpty(resp.Error)
}
