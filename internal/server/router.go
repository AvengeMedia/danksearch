package server

import (
	"fmt"
	"net"

	"github.com/AvengeMedia/danksearch/internal/config"
	"github.com/AvengeMedia/danksearch/internal/indexer"
	"github.com/AvengeMedia/danksearch/internal/log"
	"github.com/AvengeMedia/danksearch/internal/server/models"
	bleve "github.com/blevesearch/bleve/v2"
)

type IndexerInterface interface {
	Search(query string, limit int) (*bleve.SearchResult, error)
	SearchWithOptions(opts *indexer.SearchOptions) (*bleve.SearchResult, error)
	ReindexAll() error
	SyncIncremental() error
	Stats() *config.IndexStats
	ListFiles(prefix string, limit int) ([]indexer.FileEntry, int, error)
}

type WatcherInterface interface {
	Start() error
	Stop() error
	IsRunning() bool
}

type Router struct {
	indexer IndexerInterface
	watcher WatcherInterface
}

func NewRouter(indexer IndexerInterface, watcher WatcherInterface) *Router {
	return &Router{
		indexer: indexer,
		watcher: watcher,
	}
}

func (r *Router) RouteRequest(conn net.Conn, req models.Request) {
	log.Debugf("DMS API Request: method=%s id=%d", req.Method, req.ID)

	switch req.Method {
	case "ping":
		models.Respond(conn, req.ID, "pong")
	case "search":
		r.handleSearch(conn, req)
	case "reindex":
		r.handleReindex(conn, req)
	case "sync":
		r.handleSync(conn, req)
	case "stats":
		r.handleStats(conn, req)
	case "index.files":
		r.handleIndexFiles(conn, req)
	case "watch.start":
		r.handleWatchStart(conn, req)
	case "watch.stop":
		r.handleWatchStop(conn, req)
	case "watch.status":
		r.handleWatchStatus(conn, req)
	default:
		models.RespondError(conn, req.ID, fmt.Sprintf("unknown method: %s", req.Method))
	}
}

func (r *Router) handleSearch(conn net.Conn, req models.Request) {
	query, ok := req.Params["query"].(string)
	if !ok {
		models.RespondError(conn, req.ID, "query parameter required")
		return
	}

	opts := &indexer.SearchOptions{
		Query: query,
		Limit: 10,
	}

	// Parse all optional parameters
	if l, ok := req.Params["limit"].(float64); ok {
		opts.Limit = int(l)
	}
	if field, ok := req.Params["field"].(string); ok {
		opts.Field = field
	}
	if ct, ok := req.Params["content_type"].(string); ok {
		opts.ContentType = ct
	}
	if ext, ok := req.Params["extension"].(string); ok {
		opts.Extension = ext
	}
	if fuzzy, ok := req.Params["fuzzy"].(bool); ok {
		opts.Fuzzy = fuzzy
	}
	if sortBy, ok := req.Params["sort_by"].(string); ok {
		opts.SortBy = sortBy
	}
	if desc, ok := req.Params["sort_desc"].(bool); ok {
		opts.SortDesc = desc
	}
	if minSize, ok := req.Params["min_size"].(float64); ok {
		opts.MinSize = int64(minSize)
	}
	if maxSize, ok := req.Params["max_size"].(float64); ok {
		opts.MaxSize = int64(maxSize)
	}
	if modAfter, ok := req.Params["modified_after"].(string); ok {
		opts.ModifiedAfter = modAfter
	}
	if facets, ok := req.Params["facets"].([]any); ok {
		opts.Facets = make([]string, len(facets))
		for i, f := range facets {
			if s, ok := f.(string); ok {
				opts.Facets[i] = s
			}
		}
	}

	result, err := r.indexer.SearchWithOptions(opts)
	if err != nil {
		models.RespondError(conn, req.ID, fmt.Sprintf("search failed: %v", err))
		return
	}

	models.Respond(conn, req.ID, result)
}

func (r *Router) handleReindex(conn net.Conn, req models.Request) {
	go func() {
		if err := r.indexer.ReindexAll(); err != nil {
			log.Errorf("reindex failed: %v", err)
		}
	}()

	models.Respond(conn, req.ID, map[string]string{"status": "reindexing started"})
}

func (r *Router) handleSync(conn net.Conn, req models.Request) {
	go func() {
		if err := r.indexer.SyncIncremental(); err != nil {
			log.Errorf("sync failed: %v", err)
		}
	}()

	models.Respond(conn, req.ID, map[string]string{"status": "incremental sync started"})
}

func (r *Router) handleStats(conn net.Conn, req models.Request) {
	stats := r.indexer.Stats()
	models.Respond(conn, req.ID, stats)
}

func (r *Router) handleWatchStart(conn net.Conn, req models.Request) {
	if r.watcher.IsRunning() {
		models.RespondError(conn, req.ID, "watcher already running")
		return
	}

	if err := r.watcher.Start(); err != nil {
		models.RespondError(conn, req.ID, fmt.Sprintf("failed to start watcher: %v", err))
		return
	}

	models.Respond(conn, req.ID, map[string]string{"status": "watcher started"})
}

func (r *Router) handleWatchStop(conn net.Conn, req models.Request) {
	if !r.watcher.IsRunning() {
		models.RespondError(conn, req.ID, "watcher not running")
		return
	}

	if err := r.watcher.Stop(); err != nil {
		models.RespondError(conn, req.ID, fmt.Sprintf("failed to stop watcher: %v", err))
		return
	}

	models.Respond(conn, req.ID, map[string]string{"status": "watcher stopped"})
}

func (r *Router) handleWatchStatus(conn net.Conn, req models.Request) {
	status := "stopped"
	if r.watcher.IsRunning() {
		status = "running"
	}

	models.Respond(conn, req.ID, map[string]string{"status": status})
}

func (r *Router) handleIndexFiles(conn net.Conn, req models.Request) {
	prefix, _ := req.Params["prefix"].(string)
	limit := 100
	if l, ok := req.Params["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	files, total, err := r.indexer.ListFiles(prefix, limit)
	if err != nil {
		models.RespondError(conn, req.ID, fmt.Sprintf("list files failed: %v", err))
		return
	}

	models.Respond(conn, req.ID, map[string]any{
		"files": files,
		"total": total,
	})
}
