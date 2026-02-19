package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AvengeMedia/danksearch/internal/config"
	"github.com/AvengeMedia/danksearch/internal/indexer"
	bleve "github.com/blevesearch/bleve/v2"
)

type mockHTTPIndexer struct{}

func (m *mockHTTPIndexer) Search(query string, limit int) (*bleve.SearchResult, error) {
	return &bleve.SearchResult{Total: 0}, nil
}

func (m *mockHTTPIndexer) SearchWithOptions(opts *indexer.SearchOptions) (*bleve.SearchResult, error) {
	return &bleve.SearchResult{Total: 0}, nil
}

func (m *mockHTTPIndexer) SearchAll(opts *indexer.SearchOptions) (*indexer.SearchResult, error) {
	return &indexer.SearchResult{SearchResult: &bleve.SearchResult{Total: 0}}, nil
}

func (m *mockHTTPIndexer) ReindexAll() error {
	return nil
}

func (m *mockHTTPIndexer) SyncIncremental() error {
	return nil
}

func (m *mockHTTPIndexer) Stats() *config.IndexStats {
	return &config.IndexStats{}
}

type mockHTTPWatcher struct {
	running bool
}

func (m *mockHTTPWatcher) Start() error {
	m.running = true
	return nil
}

func (m *mockHTTPWatcher) Stop() error {
	m.running = false
	return nil
}

func (m *mockHTTPWatcher) IsRunning() bool {
	return m.running
}

func TestNewHTTP(t *testing.T) {
	idx := &mockHTTPIndexer{}
	w := &mockHTTPWatcher{}

	srv := NewHTTP(":8080", idx, w)

	if srv == nil {
		t.Fatal("NewHTTP() returned nil")
	}

	if srv.server == nil {
		t.Error("server should not be nil")
	}

	if srv.server.Addr != ":8080" {
		t.Errorf("Addr = %v, want :8080", srv.server.Addr)
	}
}

func TestHTTPServer_Routes(t *testing.T) {
	idx := &mockHTTPIndexer{}
	w := &mockHTTPWatcher{}

	srv := NewHTTP(":8080", idx, w)

	tests := []struct {
		name   string
		path   string
		method string
		status int
	}{
		{
			name:   "health endpoint",
			path:   "/health",
			method: http.MethodGet,
			status: http.StatusOK,
		},
		{
			name:   "search endpoint",
			path:   "/search?q=test",
			method: http.MethodGet,
			status: http.StatusOK,
		},
		{
			name:   "stats endpoint",
			path:   "/stats",
			method: http.MethodGet,
			status: http.StatusOK,
		},
		{
			name:   "watch status endpoint",
			path:   "/watch/status",
			method: http.MethodGet,
			status: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			srv.server.Handler.ServeHTTP(rec, req)

			if rec.Code != tt.status {
				t.Errorf("status = %v, want %v", rec.Code, tt.status)
			}
		})
	}
}

func TestHTTPServer_Shutdown(t *testing.T) {
	idx := &mockHTTPIndexer{}
	w := &mockHTTPWatcher{}

	srv := NewHTTP(":0", idx, w)

	go func() {
		srv.Start()
	}()

	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}
