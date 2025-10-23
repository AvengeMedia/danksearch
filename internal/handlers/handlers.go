package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/AvengeMedia/danksearch/internal/config"
	"github.com/AvengeMedia/danksearch/internal/log"
	bleve "github.com/blevesearch/bleve/v2"
)

type IndexerInterface interface {
	Search(query string, limit int) (*bleve.SearchResult, error)
	ReindexAll() error
	Stats() *config.IndexStats
}

type WatcherInterface interface {
	Start() error
	Stop() error
	IsRunning() bool
}

type Handler struct {
	indexer IndexerInterface
	watcher WatcherInterface
}

func New(indexer IndexerInterface, watcher WatcherInterface) *Handler {
	return &Handler{
		indexer: indexer,
		watcher: watcher,
	}
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "query parameter 'q' required", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	result, err := h.indexer.Search(query, limit)
	if err != nil {
		log.Errorf("search failed: %v", err)
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) Reindex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	go func() {
		if err := h.indexer.ReindexAll(); err != nil {
			log.Errorf("reindex failed: %v", err)
		}
	}()

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "reindexing started"})
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	stats := h.indexer.Stats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) WatchStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.watcher.IsRunning() {
		http.Error(w, "watcher already running", http.StatusConflict)
		return
	}

	if err := h.watcher.Start(); err != nil {
		log.Errorf("failed to start watcher: %v", err)
		http.Error(w, "failed to start watcher", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "watcher started"})
}

func (h *Handler) WatchStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.watcher.IsRunning() {
		http.Error(w, "watcher not running", http.StatusConflict)
		return
	}

	if err := h.watcher.Stop(); err != nil {
		log.Errorf("failed to stop watcher: %v", err)
		http.Error(w, "failed to stop watcher", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "watcher stopped"})
}

func (h *Handler) WatchStatus(w http.ResponseWriter, r *http.Request) {
	status := "stopped"
	if h.watcher.IsRunning() {
		status = "running"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": status})
}
