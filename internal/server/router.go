package server

import (
	"context"
	"fmt"

	"github.com/AvengeMedia/dankgo/ipc"
	"github.com/AvengeMedia/dankgo/ipc/params"
	"github.com/AvengeMedia/dankgo/log"
	"github.com/AvengeMedia/danksearch/internal/config"
	"github.com/AvengeMedia/danksearch/internal/indexer"
	bleve "github.com/blevesearch/bleve/v2"
)

type IndexerInterface interface {
	Search(query string, limit int) (*bleve.SearchResult, error)
	SearchWithOptions(opts *indexer.SearchOptions) (*bleve.SearchResult, error)
	SearchAll(opts *indexer.SearchOptions) (*indexer.SearchResult, error)
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

func (r *Router) Handle(_ context.Context, w *ipc.ConnWriter, req ipc.Request, _ *ipc.Subscriber) {
	log.Debugf("DMS API Request: method=%s id=%d", req.Method, req.ID)

	switch req.Method {
	case "search":
		r.handleSearch(w, req)
	case "reindex":
		r.handleReindex(w, req)
	case "sync":
		r.handleSync(w, req)
	case "stats":
		r.handleStats(w, req)
	case "index.files":
		r.handleIndexFiles(w, req)
	case "watch.start":
		r.handleWatchStart(w, req)
	case "watch.stop":
		r.handleWatchStop(w, req)
	case "watch.status":
		r.handleWatchStatus(w, req)
	default:
		ipc.RespondError(w, req.ID, fmt.Sprintf("unknown method: %s", req.Method))
	}
}

func (r *Router) handleSearch(w *ipc.ConnWriter, req ipc.Request) {
	query, err := params.String(req.Params, "query")
	if err != nil {
		ipc.RespondError(w, req.ID, "query parameter required")
		return
	}

	opts := &indexer.SearchOptions{
		Query:         query,
		Limit:         params.IntOpt(req.Params, "limit", 10),
		Field:         params.StringOpt(req.Params, "field", ""),
		ContentType:   params.StringOpt(req.Params, "content_type", ""),
		Extension:     params.StringOpt(req.Params, "extension", ""),
		Fuzzy:         params.BoolOpt(req.Params, "fuzzy", false),
		SortBy:        params.StringOpt(req.Params, "sort_by", ""),
		SortDesc:      params.BoolOpt(req.Params, "sort_desc", false),
		MinSize:       int64(params.FloatOpt(req.Params, "min_size", 0)),
		MaxSize:       int64(params.FloatOpt(req.Params, "max_size", 0)),
		ModifiedAfter: params.StringOpt(req.Params, "modified_after", ""),
		Folder:        params.StringOpt(req.Params, "folder", ""),
		Type:          params.StringOpt(req.Params, "type", ""),
		Facets:        params.StringSlice(req.Params, "facets"),
	}

	if opts.Type == "all" {
		result, err := r.indexer.SearchAll(opts)
		if err != nil {
			ipc.RespondError(w, req.ID, fmt.Sprintf("search failed: %v", err))
			return
		}
		ipc.Respond(w, req.ID, result)
		return
	}

	result, err := r.indexer.SearchWithOptions(opts)
	if err != nil {
		ipc.RespondError(w, req.ID, fmt.Sprintf("search failed: %v", err))
		return
	}

	ipc.Respond(w, req.ID, &indexer.SearchResult{SearchResult: result})
}

func (r *Router) handleReindex(w *ipc.ConnWriter, req ipc.Request) {
	go func() {
		if err := r.indexer.ReindexAll(); err != nil {
			log.Errorf("reindex failed: %v", err)
		}
	}()

	ipc.Respond(w, req.ID, map[string]string{"status": "reindexing started"})
}

func (r *Router) handleSync(w *ipc.ConnWriter, req ipc.Request) {
	go func() {
		if err := r.indexer.SyncIncremental(); err != nil {
			log.Errorf("sync failed: %v", err)
		}
	}()

	ipc.Respond(w, req.ID, map[string]string{"status": "incremental sync started"})
}

func (r *Router) handleStats(w *ipc.ConnWriter, req ipc.Request) {
	ipc.Respond(w, req.ID, r.indexer.Stats())
}

func (r *Router) handleWatchStart(w *ipc.ConnWriter, req ipc.Request) {
	if r.watcher.IsRunning() {
		ipc.RespondError(w, req.ID, "watcher already running")
		return
	}

	if err := r.watcher.Start(); err != nil {
		ipc.RespondError(w, req.ID, fmt.Sprintf("failed to start watcher: %v", err))
		return
	}

	ipc.Respond(w, req.ID, map[string]string{"status": "watcher started"})
}

func (r *Router) handleWatchStop(w *ipc.ConnWriter, req ipc.Request) {
	if !r.watcher.IsRunning() {
		ipc.RespondError(w, req.ID, "watcher not running")
		return
	}

	if err := r.watcher.Stop(); err != nil {
		ipc.RespondError(w, req.ID, fmt.Sprintf("failed to stop watcher: %v", err))
		return
	}

	ipc.Respond(w, req.ID, map[string]string{"status": "watcher stopped"})
}

func (r *Router) handleWatchStatus(w *ipc.ConnWriter, req ipc.Request) {
	status := "stopped"
	if r.watcher.IsRunning() {
		status = "running"
	}

	ipc.Respond(w, req.ID, map[string]string{"status": status})
}

func (r *Router) handleIndexFiles(w *ipc.ConnWriter, req ipc.Request) {
	prefix := params.StringOpt(req.Params, "prefix", "")

	limit := 100
	if l := params.IntOpt(req.Params, "limit", 0); l > 0 {
		limit = l
	}

	files, total, err := r.indexer.ListFiles(prefix, limit)
	if err != nil {
		ipc.RespondError(w, req.ID, fmt.Sprintf("list files failed: %v", err))
		return
	}

	ipc.Respond(w, req.ID, map[string]any{
		"files": files,
		"total": total,
	})
}
