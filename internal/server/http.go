package server

import (
	"context"
	"net/http"
	"time"

	"github.com/AvengeMedia/danksearch/internal/api"
	"github.com/AvengeMedia/danksearch/internal/log"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type HTTPServer struct {
	server *http.Server
	api    huma.API
}

func NewHumaConfig(title, version string) huma.Config {
	schemaPrefix := "#/components/schemas/"
	schemasPath := "/schemas"

	registry := huma.NewMapRegistry(schemaPrefix, huma.DefaultSchemaNamer)

	return huma.Config{
		OpenAPI: &huma.OpenAPI{
			OpenAPI: "3.1.0",
			Info: &huma.Info{
				Title:       title,
				Version:     version,
				Description: "Desktop filesystem search service with Bleve indexing",
			},
			Components: &huma.Components{
				Schemas: registry,
			},
		},
		OpenAPIPath:   "/openapi",
		DocsPath:      "/docs",
		SchemasPath:   schemasPath,
		Formats:       huma.DefaultFormats,
		DefaultFormat: "application/json",
	}
}

func NewHTTP(addr string, indexer api.IndexerInterface, watcher api.WatcherInterface) *HTTPServer {
	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequestID)
		r.Use(middleware.RealIP)
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)
		r.Use(middleware.Timeout(30 * time.Second))

		config := NewHumaConfig("DankSearch API", "1.0.0")
		config.DocsPath = ""
		humaAPI := humachi.New(r, config)

		r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<!doctype html>
<html>
	<head>
		<title>DankSearch API Reference</title>
		<meta charset="utf-8" />
		<meta name="viewport" content="width=device-width, initial-scale=1" />
	</head>
	<body>
		<script
			id="api-reference"
			data-url="/openapi.json"></script>
		<script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
	</body>
</html>`))
		})

		srv := &api.Server{
			Indexer: indexer,
			Watcher: watcher,
		}

		api.RegisterHandlers(srv, humaAPI)
	})

	return &HTTPServer{
		server: &http.Server{
			Addr:         addr,
			Handler:      r,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

func (s *HTTPServer) Start() error {
	log.Infof("HTTP server listening on %s", s.server.Addr)
	log.Infof("API Documentation: http://localhost%s/docs", s.server.Addr)
	log.Infof("OpenAPI Spec: http://localhost%s/openapi.json", s.server.Addr)
	log.Infof("Health Check: http://localhost%s/health", s.server.Addr)

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	log.Infof("shutting down HTTP server")
	return s.server.Shutdown(ctx)
}
