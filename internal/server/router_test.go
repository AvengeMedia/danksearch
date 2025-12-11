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
)

type mockRouterIndexer struct{}

func (m *mockRouterIndexer) Search(query string, limit int) (*bleve.SearchResult, error) {
	return &bleve.SearchResult{Total: 5}, nil
}

func (m *mockRouterIndexer) SearchWithOptions(opts *indexer.SearchOptions) (*bleve.SearchResult, error) {
	return &bleve.SearchResult{Total: 5}, nil
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

func TestRouter_Ping(t *testing.T) {
	idx := &mockRouterIndexer{}
	w := &mockRouterWatcher{}
	router := NewRouter(idx, w)

	conn := &mockConn{}
	req := models.Request{
		ID:     1,
		Method: "ping",
	}

	router.RouteRequest(conn, req)

	var resp models.Response[string]
	if err := json.Unmarshal(conn.written, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.ID != 1 {
		t.Errorf("ID = %v, want 1", resp.ID)
	}

	if resp.Result == nil || *resp.Result != "pong" {
		t.Errorf("Result = %v, want pong", resp.Result)
	}
}

func TestRouter_Search(t *testing.T) {
	idx := &mockRouterIndexer{}
	w := &mockRouterWatcher{}
	router := NewRouter(idx, w)

	conn := &mockConn{}
	req := models.Request{
		ID:     2,
		Method: "search",
		Params: map[string]any{
			"query": "test",
			"limit": float64(10),
		},
	}

	router.RouteRequest(conn, req)

	var resp models.Response[bleve.SearchResult]
	if err := json.Unmarshal(conn.written, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.ID != 2 {
		t.Errorf("ID = %v, want 2", resp.ID)
	}

	if resp.Result == nil {
		t.Fatal("Result should not be nil")
	}

	if resp.Result.Total != 5 {
		t.Errorf("Total = %v, want 5", resp.Result.Total)
	}
}

func TestRouter_Stats(t *testing.T) {
	idx := &mockRouterIndexer{}
	w := &mockRouterWatcher{}
	router := NewRouter(idx, w)

	conn := &mockConn{}
	req := models.Request{
		ID:     3,
		Method: "stats",
	}

	router.RouteRequest(conn, req)

	var resp models.Response[config.IndexStats]
	if err := json.Unmarshal(conn.written, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Result == nil {
		t.Fatal("Result should not be nil")
	}

	if resp.Result.TotalFiles != 100 {
		t.Errorf("TotalFiles = %v, want 100", resp.Result.TotalFiles)
	}
}

func TestRouter_WatchStart(t *testing.T) {
	idx := &mockRouterIndexer{}
	w := &mockRouterWatcher{}
	router := NewRouter(idx, w)

	conn := &mockConn{}
	req := models.Request{
		ID:     4,
		Method: "watch.start",
	}

	router.RouteRequest(conn, req)

	var resp models.Response[map[string]string]
	if err := json.Unmarshal(conn.written, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Result == nil {
		t.Fatal("Result should not be nil")
	}

	if (*resp.Result)["status"] != "watcher started" {
		t.Errorf("status = %v, want 'watcher started'", (*resp.Result)["status"])
	}

	if !w.running {
		t.Error("watcher should be running")
	}
}

func TestRouter_WatchStop(t *testing.T) {
	idx := &mockRouterIndexer{}
	w := &mockRouterWatcher{running: true}
	router := NewRouter(idx, w)

	conn := &mockConn{}
	req := models.Request{
		ID:     5,
		Method: "watch.stop",
	}

	router.RouteRequest(conn, req)

	var resp models.Response[map[string]string]
	if err := json.Unmarshal(conn.written, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Result == nil {
		t.Fatal("Result should not be nil")
	}

	if (*resp.Result)["status"] != "watcher stopped" {
		t.Errorf("status = %v, want 'watcher stopped'", (*resp.Result)["status"])
	}

	if w.running {
		t.Error("watcher should not be running")
	}
}

func TestRouter_WatchStatus(t *testing.T) {
	tests := []struct {
		name     string
		running  bool
		expected string
	}{
		{
			name:     "running",
			running:  true,
			expected: "running",
		},
		{
			name:     "stopped",
			running:  false,
			expected: "stopped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := &mockRouterIndexer{}
			w := &mockRouterWatcher{running: tt.running}
			router := NewRouter(idx, w)

			conn := &mockConn{}
			req := models.Request{
				ID:     6,
				Method: "watch.status",
			}

			router.RouteRequest(conn, req)

			var resp models.Response[map[string]string]
			if err := json.Unmarshal(conn.written, &resp); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if resp.Result == nil {
				t.Fatal("Result should not be nil")
			}

			if (*resp.Result)["status"] != tt.expected {
				t.Errorf("status = %v, want %v", (*resp.Result)["status"], tt.expected)
			}
		})
	}
}

func TestRouter_Reindex(t *testing.T) {
	idx := &mockRouterIndexer{}
	w := &mockRouterWatcher{}
	router := NewRouter(idx, w)

	conn := &mockConn{}
	req := models.Request{
		ID:     7,
		Method: "reindex",
	}

	router.RouteRequest(conn, req)

	time.Sleep(50 * time.Millisecond)

	var resp models.Response[map[string]string]
	if err := json.Unmarshal(conn.written, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Result == nil {
		t.Fatal("Result should not be nil")
	}

	if (*resp.Result)["status"] != "reindexing started" {
		t.Errorf("status = %v, want 'reindexing started'", (*resp.Result)["status"])
	}
}

func TestRouter_UnknownMethod(t *testing.T) {
	idx := &mockRouterIndexer{}
	w := &mockRouterWatcher{}
	router := NewRouter(idx, w)

	conn := &mockConn{}
	req := models.Request{
		ID:     8,
		Method: "unknown",
	}

	router.RouteRequest(conn, req)

	var resp models.Response[any]
	if err := json.Unmarshal(conn.written, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error == "" {
		t.Error("Error should not be empty")
	}
}
