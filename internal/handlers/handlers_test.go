package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AvengeMedia/danksearch/internal/config"
	mocks_handlers "github.com/AvengeMedia/danksearch/internal/mocks/handlers"
	bleve "github.com/blevesearch/bleve/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type HandlersSuite struct {
	suite.Suite
	indexer *mocks_handlers.MockIndexerInterface
	watcher *mocks_handlers.MockWatcherInterface
	handler *Handler
}

func TestHandlersSuite(t *testing.T) {
	suite.Run(t, new(HandlersSuite))
}

func (s *HandlersSuite) SetupTest() {
	s.indexer = mocks_handlers.NewMockIndexerInterface(s.T())
	s.watcher = mocks_handlers.NewMockWatcherInterface(s.T())
	s.handler = New(s.indexer, s.watcher)
}

func (s *HandlersSuite) TestSearch() {
	tests := []struct {
		name           string
		query          string
		mockResult     *bleve.SearchResult
		mockError      error
		expectedStatus int
	}{
		{"successful search", "test", &bleve.SearchResult{Total: 5}, nil, http.StatusOK},
		{"missing query parameter", "", nil, nil, http.StatusBadRequest},
		{"search error", "test", nil, errors.New("search failed"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.query != "" {
				s.indexer.EXPECT().Search(tt.query, mock.Anything).Return(tt.mockResult, tt.mockError).Once()
			}

			req := httptest.NewRequest(http.MethodGet, "/search?q="+tt.query, nil)
			rec := httptest.NewRecorder()
			s.handler.Search(rec, req)

			s.Equal(tt.expectedStatus, rec.Code)
		})
	}
}

func (s *HandlersSuite) TestReindex() {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{"successful reindex", http.MethodPost, http.StatusAccepted},
		{"wrong method", http.MethodGet, http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.indexer.EXPECT().ReindexAll().Return(nil).Maybe()

			req := httptest.NewRequest(tt.method, "/reindex", nil)
			rec := httptest.NewRecorder()
			s.handler.Reindex(rec, req)

			s.Equal(tt.expectedStatus, rec.Code)
		})
	}
}

func (s *HandlersSuite) TestStats() {
	stats := &config.IndexStats{TotalFiles: 100, TotalBytes: 1024}
	s.indexer.EXPECT().Stats().Return(stats).Once()

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	rec := httptest.NewRecorder()
	s.handler.Stats(rec, req)

	s.Equal(http.StatusOK, rec.Code)

	var result config.IndexStats
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&result))
	s.Equal(stats.TotalFiles, result.TotalFiles)
}

func (s *HandlersSuite) TestWatchStart() {
	tests := []struct {
		name           string
		method         string
		isRunning      bool
		startError     error
		expectedStatus int
	}{
		{"successful start", http.MethodPost, false, nil, http.StatusOK},
		{"already running", http.MethodPost, true, nil, http.StatusConflict},
		{"wrong method", http.MethodGet, false, nil, http.StatusMethodNotAllowed},
		{"start error", http.MethodPost, false, errors.New("failed"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.method == http.MethodPost {
				s.watcher.EXPECT().IsRunning().Return(tt.isRunning).Once()
				if !tt.isRunning {
					s.watcher.EXPECT().Start().Return(tt.startError).Once()
				}
			}

			req := httptest.NewRequest(tt.method, "/watch/start", nil)
			rec := httptest.NewRecorder()
			s.handler.WatchStart(rec, req)

			s.Equal(tt.expectedStatus, rec.Code)
		})
	}
}

func (s *HandlersSuite) TestWatchStop() {
	tests := []struct {
		name           string
		method         string
		isRunning      bool
		stopError      error
		expectedStatus int
	}{
		{"successful stop", http.MethodPost, true, nil, http.StatusOK},
		{"not running", http.MethodPost, false, nil, http.StatusConflict},
		{"wrong method", http.MethodGet, true, nil, http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.method == http.MethodPost {
				s.watcher.EXPECT().IsRunning().Return(tt.isRunning).Once()
				if tt.isRunning {
					s.watcher.EXPECT().Stop().Return(tt.stopError).Once()
				}
			}

			req := httptest.NewRequest(tt.method, "/watch/stop", nil)
			rec := httptest.NewRecorder()
			s.handler.WatchStop(rec, req)

			s.Equal(tt.expectedStatus, rec.Code)
		})
	}
}

func (s *HandlersSuite) TestWatchStatus() {
	tests := []struct {
		name      string
		isRunning bool
		expected  string
	}{
		{"watcher running", true, "running"},
		{"watcher stopped", false, "stopped"},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.watcher.EXPECT().IsRunning().Return(tt.isRunning).Once()

			req := httptest.NewRequest(http.MethodGet, "/watch/status", nil)
			rec := httptest.NewRecorder()
			s.handler.WatchStatus(rec, req)

			s.Equal(http.StatusOK, rec.Code)

			var result map[string]string
			s.Require().NoError(json.NewDecoder(rec.Body).Decode(&result))
			s.Equal(tt.expected, result["status"])
		})
	}
}
