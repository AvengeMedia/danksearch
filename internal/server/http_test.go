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
	"github.com/stretchr/testify/suite"
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

type HTTPSuite struct {
	suite.Suite
}

func TestHTTPSuite(t *testing.T) {
	suite.Run(t, new(HTTPSuite))
}

func (s *HTTPSuite) TestNewHTTP() {
	srv := NewHTTP(":8080", &mockHTTPIndexer{}, &mockHTTPWatcher{})
	s.Require().NotNil(srv)
	s.Equal(":8080", srv.Addr())
}

func (s *HTTPSuite) TestRoutes() {
	handler := newHTTPHandler(&mockHTTPIndexer{}, &mockHTTPWatcher{})

	tests := []struct {
		name   string
		path   string
		method string
		status int
	}{
		{"health endpoint", "/health", http.MethodGet, http.StatusOK},
		{"search endpoint", "/search?q=test", http.MethodGet, http.StatusOK},
		{"stats endpoint", "/stats", http.MethodGet, http.StatusOK},
		{"watch status endpoint", "/watch/status", http.MethodGet, http.StatusOK},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			s.Equal(tt.status, rec.Code)
		})
	}
}

func (s *HTTPSuite) TestShutdown() {
	srv := NewHTTP("127.0.0.1:0", &mockHTTPIndexer{}, &mockHTTPWatcher{})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- srv.Serve(ctx)
	}()
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		s.NoError(err)
	case <-time.After(5 * time.Second):
		s.Fail("server did not shut down")
	}
}
