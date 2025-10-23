package api

import (
	"context"

	"github.com/AvengeMedia/danksearch/internal/config"
	"github.com/AvengeMedia/danksearch/internal/indexer"
	"github.com/AvengeMedia/danksearch/internal/log"
	bleve "github.com/blevesearch/bleve/v2"
	"github.com/danielgtaylor/huma/v2"
)

type IndexerInterface interface {
	Search(query string, limit int) (*bleve.SearchResult, error)
	SearchWithOptions(opts *indexer.SearchOptions) (*bleve.SearchResult, error)
	ReindexAll() error
	SyncIncremental() error
	Stats() *config.IndexStats
}

type WatcherInterface interface {
	Start() error
	Stop() error
	IsRunning() bool
}

type Server struct {
	Indexer IndexerInterface
	Watcher WatcherInterface
}

type SearchInput struct {
	Query         string   `query:"q" minLength:"1" doc:"Search query" example:"mountain"`
	Limit         int      `query:"limit" default:"10" minimum:"1" maximum:"500" doc:"Maximum results"`
	Field         string   `query:"field" enum:"filename,body,title," doc:"Search specific field (empty for all)"`
	ContentType   string   `query:"content_type" doc:"Filter by MIME type" example:"image/jpeg"`
	Extension     string   `query:"ext" doc:"Filter by file extension" example:".jpg"`
	Fuzzy         bool     `query:"fuzzy" doc:"Enable fuzzy matching for typos"`
	SortBy        string   `query:"sort" enum:"score,mtime,size,filename," default:"score" doc:"Sort results by field"`
	SortDesc      bool     `query:"desc" default:"true" doc:"Sort descending"`
	MinSize       int64    `query:"min_size" doc:"Minimum file size in bytes"`
	MaxSize       int64    `query:"max_size" doc:"Maximum file size in bytes"`
	ModifiedAfter string   `query:"modified_after" doc:"Filter by modification date (RFC3339)" example:"2024-01-01T00:00:00Z"`
	Facets        []string `query:"facets" doc:"Enable facets for fields" example:"content_type"`
}

type SearchOutput struct {
	Body *bleve.SearchResult
}

type ReindexOutput struct {
	Body struct {
		Status string `json:"status" example:"reindexing started"`
	}
}

type StatsOutput struct {
	Body *config.IndexStats
}

type WatchStatusOutput struct {
	Body struct {
		Status string `json:"status" enum:"running,stopped" example:"running"`
	}
}

type WatchActionOutput struct {
	Body struct {
		Status string `json:"status" example:"watcher started"`
	}
}

func RegisterHandlers(srv *Server, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "search",
		Summary:     "Search indexed files",
		Description: "Advanced search with filters, facets, sorting, and metadata queries",
		Method:      "GET",
		Path:        "/search",
		Tags:        []string{"Search"},
	}, func(ctx context.Context, input *SearchInput) (*SearchOutput, error) {
		opts := &indexer.SearchOptions{
			Query:         input.Query,
			Limit:         input.Limit,
			Field:         input.Field,
			ContentType:   input.ContentType,
			Extension:     input.Extension,
			Fuzzy:         input.Fuzzy,
			SortBy:        input.SortBy,
			SortDesc:      input.SortDesc,
			MinSize:       input.MinSize,
			MaxSize:       input.MaxSize,
			ModifiedAfter: input.ModifiedAfter,
			Facets:        input.Facets,
		}

		result, err := srv.Indexer.SearchWithOptions(opts)
		if err != nil {
			return nil, huma.Error400BadRequest("search failed", err)
		}

		return &SearchOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "reindex",
		Summary:     "Trigger full reindex",
		Description: "Start a full reindex of all files (async operation)",
		Method:      "POST",
		Path:        "/reindex",
		Tags:        []string{"Index"},
	}, func(ctx context.Context, input *struct{}) (*ReindexOutput, error) {
		go func() {
			if err := srv.Indexer.ReindexAll(); err != nil {
				log.Errorf("reindex failed: %v", err)
			}
		}()

		return &ReindexOutput{
			Body: struct {
				Status string `json:"status" example:"reindexing started"`
			}{
				Status: "reindexing started",
			},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "sync",
		Summary:     "Incremental sync",
		Description: "Add new/updated files, remove deleted files (async operation)",
		Method:      "POST",
		Path:        "/sync",
		Tags:        []string{"Index"},
	}, func(ctx context.Context, input *struct{}) (*ReindexOutput, error) {
		go func() {
			if err := srv.Indexer.SyncIncremental(); err != nil {
				log.Errorf("sync failed: %v", err)
			}
		}()

		return &ReindexOutput{
			Body: struct {
				Status string `json:"status" example:"reindexing started"`
			}{
				Status: "incremental sync started",
			},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "stats",
		Summary:     "Get index statistics",
		Description: "Returns statistics about the indexed files",
		Method:      "GET",
		Path:        "/stats",
		Tags:        []string{"Index"},
	}, func(ctx context.Context, input *struct{}) (*StatsOutput, error) {
		stats := srv.Indexer.Stats()
		return &StatsOutput{Body: stats}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "watchStart",
		Summary:     "Start file watcher",
		Description: "Enable real-time file watching for incremental updates",
		Method:      "POST",
		Path:        "/watch/start",
		Tags:        []string{"Watch"},
	}, func(ctx context.Context, input *struct{}) (*WatchActionOutput, error) {
		if srv.Watcher.IsRunning() {
			return nil, huma.Error409Conflict("watcher already running")
		}

		if err := srv.Watcher.Start(); err != nil {
			return nil, huma.Error500InternalServerError("failed to start watcher", err)
		}

		return &WatchActionOutput{
			Body: struct {
				Status string `json:"status" example:"watcher started"`
			}{
				Status: "watcher started",
			},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "watchStop",
		Summary:     "Stop file watcher",
		Description: "Disable real-time file watching",
		Method:      "POST",
		Path:        "/watch/stop",
		Tags:        []string{"Watch"},
	}, func(ctx context.Context, input *struct{}) (*WatchActionOutput, error) {
		if !srv.Watcher.IsRunning() {
			return nil, huma.Error409Conflict("watcher not running")
		}

		if err := srv.Watcher.Stop(); err != nil {
			return nil, huma.Error500InternalServerError("failed to stop watcher", err)
		}

		return &WatchActionOutput{
			Body: struct {
				Status string `json:"status" example:"watcher started"`
			}{
				Status: "watcher stopped",
			},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "watchStatus",
		Summary:     "Get watcher status",
		Description: "Check if file watcher is currently running",
		Method:      "GET",
		Path:        "/watch/status",
		Tags:        []string{"Watch"},
	}, func(ctx context.Context, input *struct{}) (*WatchStatusOutput, error) {
		status := "stopped"
		if srv.Watcher.IsRunning() {
			status = "running"
		}

		return &WatchStatusOutput{
			Body: struct {
				Status string `json:"status" enum:"running,stopped" example:"running"`
			}{
				Status: status,
			},
		}, nil
	})
}
