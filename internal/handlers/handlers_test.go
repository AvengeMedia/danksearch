package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AvengeMedia/danksearch/internal/config"
	bleve "github.com/blevesearch/bleve/v2"
	"github.com/stretchr/testify/suite"
)

type mockIndexer struct {
	searchResult *bleve.SearchResult
	searchError  error
	reindexError error
	stats        *config.IndexStats
}

func (m *mockIndexer) Search(query string, limit int) (*bleve.SearchResult, error) {
	return m.searchResult, m.searchError
}

func (m *mockIndexer) ReindexAll() error {
	return m.reindexError
}

func (m *mockIndexer) Stats() *config.IndexStats {
	return m.stats
}

type mockWatcher struct {
	running    bool
	startError error
	stopError  error
}

func (m *mockWatcher) Start() error {
	if m.startError != nil {
		return m.startError
	}
	m.running = true
	return nil
}

func (m *mockWatcher) Stop() error {
	if m.stopError != nil {
		return m.stopError
	}
	m.running = false
	return nil
}

func (m *mockWatcher) IsRunning() bool {
	return m.running
}

type HandlersSuite struct {
	suite.Suite
}

func TestHandlersSuite(t *testing.T) {
	suite.Run(t, new(HandlersSuite))
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
			idx := &mockIndexer{searchResult: tt.mockResult, searchError: tt.mockError}
			h := New(idx, &mockWatcher{})

			req := httptest.NewRequest(http.MethodGet, "/search?q="+tt.query, nil)
			rec := httptest.NewRecorder()
			h.Search(rec, req)

			s.Equal(tt.expectedStatus, rec.Code)
		})
	}
}

func (s *HandlersSuite) TestReindex() {
	tests := []struct {
		name           string
		method         string
		reindexError   error
		expectedStatus int
	}{
		{"successful reindex", http.MethodPost, nil, http.StatusAccepted},
		{"wrong method", http.MethodGet, nil, http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			idx := &mockIndexer{reindexError: tt.reindexError}
			h := New(idx, &mockWatcher{})

			req := httptest.NewRequest(tt.method, "/reindex", nil)
			rec := httptest.NewRecorder()
			h.Reindex(rec, req)

			s.Equal(tt.expectedStatus, rec.Code)
		})
	}
}

func (s *HandlersSuite) TestStats() {
	stats := &config.IndexStats{TotalFiles: 100, TotalBytes: 1024}
	idx := &mockIndexer{stats: stats}
	h := New(idx, &mockWatcher{})

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)

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
			w := &mockWatcher{running: tt.isRunning, startError: tt.startError}
			h := New(&mockIndexer{}, w)

			req := httptest.NewRequest(tt.method, "/watch/start", nil)
			rec := httptest.NewRecorder()
			h.WatchStart(rec, req)

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
			w := &mockWatcher{running: tt.isRunning, stopError: tt.stopError}
			h := New(&mockIndexer{}, w)

			req := httptest.NewRequest(tt.method, "/watch/stop", nil)
			rec := httptest.NewRecorder()
			h.WatchStop(rec, req)

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
			w := &mockWatcher{running: tt.isRunning}
			h := New(&mockIndexer{}, w)

			req := httptest.NewRequest(http.MethodGet, "/watch/status", nil)
			rec := httptest.NewRecorder()
			h.WatchStatus(rec, req)

			s.Equal(http.StatusOK, rec.Code)

			var result map[string]string
			s.Require().NoError(json.NewDecoder(rec.Body).Decode(&result))
			s.Equal(tt.expected, result["status"])
		})
	}
}
