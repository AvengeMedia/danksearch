package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/AvengeMedia/danksearch/internal/client"
	"github.com/AvengeMedia/danksearch/internal/config"
	"github.com/AvengeMedia/danksearch/internal/indexer"
	"github.com/AvengeMedia/danksearch/internal/log"
	"github.com/AvengeMedia/danksearch/internal/server"
	"github.com/AvengeMedia/danksearch/internal/watcher"
	"github.com/spf13/cobra"
)

var (
	Version   string = "dev"
	buildTime string = "unknown"
	commit    string = "unknown"

	configFile    string
	rootDir       string
	indexPath     string
	listenAddr    string
	maxFileBytes  int64
	workerCount   int
	maxDepth      int
	excludeHidden bool
	noWatch       bool
	httpOnly      bool
	socketOnly    bool

	searchLimit    int
	searchField    string
	searchExt      string
	searchFuzzy    bool
	searchSort     string
	searchSortDesc bool
	searchMinSize  int64
	searchMaxSize  int64
	searchJSON     bool
)

var rootCmd = &cobra.Command{
	Use:   "dsearch",
	Short: "Desktop filesystem search service",
	Long:  "A filesystem search service using Bleve to index and search files",
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the search service",
	RunE:  runServe,
}

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search the index",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSearch,
}

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Manage the search index",
}

var indexGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Trigger a full reindex (async)",
	RunE:  runIndexGenerate,
}

var indexSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Incremental sync - add new/updated, remove deleted files",
	RunE:  runIndexSync,
}

var indexStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show index statistics",
	RunE:  runIndexStatus,
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Manage file watcher",
}

var watchStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check watcher status",
	RunE:  runWatchStatus,
}

var watchStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start file watcher",
	RunE:  runWatchStart,
}

var watchStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop file watcher",
	RunE:  runWatchStop,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(cmd *cobra.Command, args []string) {
		log.Infof("dsearch version %s", Version)
		log.Infof("  Build time: %s", buildTime)
		log.Infof("  Commit: %s", commit)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file path (default: ~/.config/danksearch/config.toml)")

	serveCmd.Flags().StringVar(&rootDir, "root", "", "directory to index")
	serveCmd.Flags().StringVar(&indexPath, "index", "", "index storage path")
	serveCmd.Flags().StringVar(&listenAddr, "listen", "", "HTTP listen address")
	serveCmd.Flags().Int64Var(&maxFileBytes, "max-bytes", 0, "max file size to index")
	serveCmd.Flags().IntVar(&workerCount, "workers", 0, "number of indexing workers")
	serveCmd.Flags().IntVar(&maxDepth, "max-depth", -1, "maximum directory depth to traverse (0 = unlimited)")
	serveCmd.Flags().BoolVar(&excludeHidden, "exclude-hidden", false, "exclude hidden files and directories")
	serveCmd.Flags().BoolVar(&noWatch, "no-watch", false, "disable automatic file watching")
	serveCmd.Flags().BoolVar(&httpOnly, "http", false, "run HTTP server only (no unix socket)")
	serveCmd.Flags().BoolVar(&socketOnly, "socket", false, "run unix socket server only (no HTTP)")

	searchCmd.Flags().IntVarP(&searchLimit, "limit", "n", 10, "maximum number of results (0 for all)")
	searchCmd.Flags().StringVarP(&searchField, "field", "f", "", "search specific field (filename, body, title)")
	searchCmd.Flags().StringVarP(&searchExt, "ext", "e", "", "filter by file extension (e.g., .jpg)")
	searchCmd.Flags().BoolVar(&searchFuzzy, "fuzzy", false, "enable fuzzy matching for typos")
	searchCmd.Flags().StringVar(&searchSort, "sort", "score", "sort by: score, mtime, size, filename")
	searchCmd.Flags().BoolVar(&searchSortDesc, "desc", true, "sort descending")
	searchCmd.Flags().Int64Var(&searchMinSize, "min-size", 0, "minimum file size in bytes")
	searchCmd.Flags().Int64Var(&searchMaxSize, "max-size", 0, "maximum file size in bytes")
	searchCmd.Flags().BoolVar(&searchJSON, "json", false, "output results in JSON format")

	indexCmd.AddCommand(indexGenerateCmd)
	indexCmd.AddCommand(indexSyncCmd)
	indexCmd.AddCommand(indexStatusCmd)

	watchCmd.AddCommand(watchStatusCmd)
	watchCmd.AddCommand(watchStartCmd)
	watchCmd.AddCommand(watchStopCmd)

	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(indexCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(versionCmd)
}

func buildConfig() *config.Config {
	cfgPath := configFile
	if cfgPath == "" {
		cfgPath = config.GetDefaultConfigPath()
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if rootDir != "" {
		cfg.RootDir = rootDir
	}
	if indexPath != "" {
		cfg.IndexPath = indexPath
	}
	if listenAddr != "" {
		cfg.ListenAddr = listenAddr
	}
	if maxFileBytes > 0 {
		cfg.MaxFileBytes = maxFileBytes
	}
	if workerCount > 0 {
		cfg.WorkerCount = workerCount
	}
	if maxDepth >= 0 {
		cfg.MaxDepth = maxDepth
	}

	if cmd := rootCmd.Flags().Lookup("exclude-hidden"); cmd != nil && cmd.Changed {
		cfg.ExcludeHidden = excludeHidden
	}

	return cfg
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg := buildConfig()

	idx, err := indexer.New(cfg)
	if err != nil {
		return err
	}
	defer idx.Close()

	if cfg.AutoReindex && idx.ShouldReindex(cfg.ReindexIntervalHours) {
		log.Infof("auto-reindex triggered (interval: %d hours)", cfg.ReindexIntervalHours)
		go func() {
			if err := idx.ReindexAll(); err != nil {
				log.Errorf("auto-reindex failed: %v", err)
			}
		}()
	}

	w, err := watcher.New(idx, cfg)
	if err != nil {
		return err
	}

	if !noWatch {
		if err := w.Start(); err != nil {
			log.Errorf("failed to start watcher: %v", err)
			log.Infof("continuing without file watching")
		}
	}

	if httpOnly && socketOnly {
		return fmt.Errorf("cannot specify both --http and --socket flags")
	}

	runHTTP := !socketOnly
	runSocket := !httpOnly

	var httpServer *server.HTTPServer
	var unixServer *server.UnixServer

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 2)

	if runHTTP {
		httpServer = server.NewHTTP(cfg.ListenAddr, idx, w)
		go func() {
			errChan <- httpServer.Start()
		}()
	}

	if runSocket {
		router := server.NewRouter(idx, w)
		unixServer = server.NewUnix(router)
		go func() {
			errChan <- unixServer.Start()
		}()
	}

	select {
	case err := <-errChan:
		return err
	case <-sigChan:
		log.Infof("received shutdown signal")
		ctx, cancel := context.WithTimeout(context.Background(), 10)
		defer cancel()

		if w.IsRunning() {
			w.Stop()
		}

		if unixServer != nil {
			unixServer.Close()
		}
		if httpServer != nil {
			return httpServer.Shutdown(ctx)
		}
		return nil
	}
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := "*"
	if len(args) > 0 {
		query = args[0]
	}

	limit := searchLimit
	if limit == 0 {
		limit = 10000
	}

	opts := &client.SearchOptions{
		Query:     query,
		Limit:     limit,
		Field:     searchField,
		Extension: searchExt,
		Fuzzy:     searchFuzzy,
		SortBy:    searchSort,
		SortDesc:  searchSortDesc,
		MinSize:   searchMinSize,
		MaxSize:   searchMaxSize,
	}

	result, err := client.SearchWithOptions(opts)
	if err != nil {
		return err
	}

	if searchJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	log.Infof("found %d results in %s", result.Total, result.Took)
	for i, hit := range result.Hits {
		log.Infof("%d. %s (score: %.4f)", i+1, hit.ID, hit.Score)
	}

	return nil
}

func runIndexGenerate(cmd *cobra.Command, args []string) error {
	status, err := client.Reindex()
	if err != nil {
		return err
	}

	log.Infof("%s", status)
	return nil
}

func runIndexSync(cmd *cobra.Command, args []string) error {
	status, err := client.Sync()
	if err != nil {
		return err
	}

	log.Infof("%s", status)
	return nil
}

func runIndexStatus(cmd *cobra.Command, args []string) error {
	stats, err := client.Stats()
	if err != nil {
		return err
	}

	log.Infof("Index Statistics:")
	log.Infof("  Total files: %v", stats["total_files"])
	log.Infof("  Total bytes: %v", stats["total_bytes"])
	log.Infof("  Last index: %v", stats["last_index_time"])
	log.Infof("  Duration: %v", stats["index_duration"])

	return nil
}

func runWatchStatus(cmd *cobra.Command, args []string) error {
	status, err := client.WatchStatus()
	if err != nil {
		return err
	}

	log.Infof("Watcher status: %s", status)
	return nil
}

func runWatchStart(cmd *cobra.Command, args []string) error {
	status, err := client.WatchStart()
	if err != nil {
		return err
	}

	log.Infof("%s", status)
	return nil
}

func runWatchStop(cmd *cobra.Command, args []string) error {
	status, err := client.WatchStop()
	if err != nil {
		return err
	}

	log.Infof("%s", status)
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("%v", err)
	}
}
