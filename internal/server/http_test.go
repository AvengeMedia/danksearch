package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AvengeMedia/danksearch/internal/config"
	mocks_api "github.com/AvengeMedia/danksearch/internal/mocks/api"
	bleve "github.com/blevesearch/bleve/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type HTTPSuite struct {
	suite.Suite
	indexer *mocks_api.MockIndexerInterface
	watcher *mocks_api.MockWatcherInterface
}

func TestHTTPSuite(t *testing.T) {
	suite.Run(t, new(HTTPSuite))
}

func (s *HTTPSuite) SetupTest() {
	s.indexer = mocks_api.NewMockIndexerInterface(s.T())
	s.watcher = mocks_api.NewMockWatcherInterface(s.T())
}

func (s *HTTPSuite) TestNewHTTP() {
	srv := NewHTTP(":8080", s.indexer, s.watcher)
	s.Require().NotNil(srv)
	s.Equal(":8080", srv.Addr())
}

func (s *HTTPSuite) TestRoutes() {
	s.indexer.EXPECT().SearchWithOptions(mock.Anything).Return(&bleve.SearchResult{}, nil).Maybe()
	s.indexer.EXPECT().Stats().Return(&config.IndexStats{}).Maybe()
	s.watcher.EXPECT().IsRunning().Return(false).Maybe()

	handler := newHTTPHandler(s.indexer, s.watcher)

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
	srv := NewHTTP("127.0.0.1:0", s.indexer, s.watcher)

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
