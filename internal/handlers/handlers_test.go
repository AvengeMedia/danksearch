package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AvengeMedia/danksearch/internal/config"
	bleve "github.com/blevesearch/bleve/v2"
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

func TestHandler_Search(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		mockResult     *bleve.SearchResult
		mockError      error
		expectedStatus int
	}{
		{
			name:           "successful search",
			query:          "test",
			mockResult:     &bleve.SearchResult{Total: 5},
			mockError:      nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing query parameter",
			query:          "",
			mockResult:     nil,
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "search error",
			query:          "test",
			mockResult:     nil,
			mockError:      errors.New("search failed"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := &mockIndexer{
				searchResult: tt.mockResult,
				searchError:  tt.mockError,
			}
			w := &mockWatcher{}
			h := New(idx, w)

			req := httptest.NewRequest(http.MethodGet, "/search?q="+tt.query, nil)
			rec := httptest.NewRecorder()

			h.Search(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %v, want %v", rec.Code, tt.expectedStatus)
			}
		})
	}
}

func TestHandler_Reindex(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		reindexError   error
		expectedStatus int
	}{
		{
			name:           "successful reindex",
			method:         http.MethodPost,
			reindexError:   nil,
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "wrong method",
			method:         http.MethodGet,
			reindexError:   nil,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := &mockIndexer{reindexError: tt.reindexError}
			w := &mockWatcher{}
			h := New(idx, w)

			req := httptest.NewRequest(tt.method, "/reindex", nil)
			rec := httptest.NewRecorder()

			h.Reindex(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %v, want %v", rec.Code, tt.expectedStatus)
			}
		})
	}
}

func TestHandler_Stats(t *testing.T) {
	stats := &config.IndexStats{
		TotalFiles: 100,
		TotalBytes: 1024,
	}

	idx := &mockIndexer{stats: stats}
	w := &mockWatcher{}
	h := New(idx, w)

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	rec := httptest.NewRecorder()

	h.Stats(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %v, want %v", rec.Code, http.StatusOK)
	}

	var result config.IndexStats
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.TotalFiles != stats.TotalFiles {
		t.Errorf("TotalFiles = %v, want %v", result.TotalFiles, stats.TotalFiles)
	}
}

func TestHandler_WatchStart(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		isRunning      bool
		startError     error
		expectedStatus int
	}{
		{
			name:           "successful start",
			method:         http.MethodPost,
			isRunning:      false,
			startError:     nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "already running",
			method:         http.MethodPost,
			isRunning:      true,
			startError:     nil,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "wrong method",
			method:         http.MethodGet,
			isRunning:      false,
			startError:     nil,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "start error",
			method:         http.MethodPost,
			isRunning:      false,
			startError:     errors.New("failed"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := &mockIndexer{}
			w := &mockWatcher{
				running:    tt.isRunning,
				startError: tt.startError,
			}
			h := New(idx, w)

			req := httptest.NewRequest(tt.method, "/watch/start", nil)
			rec := httptest.NewRecorder()

			h.WatchStart(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %v, want %v", rec.Code, tt.expectedStatus)
			}
		})
	}
}

func TestHandler_WatchStop(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		isRunning      bool
		stopError      error
		expectedStatus int
	}{
		{
			name:           "successful stop",
			method:         http.MethodPost,
			isRunning:      true,
			stopError:      nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "not running",
			method:         http.MethodPost,
			isRunning:      false,
			stopError:      nil,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "wrong method",
			method:         http.MethodGet,
			isRunning:      true,
			stopError:      nil,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := &mockIndexer{}
			w := &mockWatcher{
				running:   tt.isRunning,
				stopError: tt.stopError,
			}
			h := New(idx, w)

			req := httptest.NewRequest(tt.method, "/watch/stop", nil)
			rec := httptest.NewRecorder()

			h.WatchStop(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %v, want %v", rec.Code, tt.expectedStatus)
			}
		})
	}
}

func TestHandler_WatchStatus(t *testing.T) {
	tests := []struct {
		name      string
		isRunning bool
		expected  string
	}{
		{
			name:      "watcher running",
			isRunning: true,
			expected:  "running",
		},
		{
			name:      "watcher stopped",
			isRunning: false,
			expected:  "stopped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := &mockIndexer{}
			w := &mockWatcher{running: tt.isRunning}
			h := New(idx, w)

			req := httptest.NewRequest(http.MethodGet, "/watch/status", nil)
			rec := httptest.NewRecorder()

			h.WatchStatus(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %v, want %v", rec.Code, http.StatusOK)
			}

			var result map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if result["status"] != tt.expected {
				t.Errorf("status = %v, want %v", result["status"], tt.expected)
			}
		})
	}
}
