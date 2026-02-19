package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"path/filepath"
	"strings"

	"github.com/AvengeMedia/danksearch/internal/client"
	"github.com/AvengeMedia/danksearch/internal/config"
	"github.com/AvengeMedia/danksearch/internal/indexer"
	"github.com/AvengeMedia/danksearch/internal/log"
	"github.com/AvengeMedia/danksearch/internal/metastore"
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

	filesLimit            int
	searchFolder          string
	searchExifMake        string
	searchExifModel       string
	searchExifDateAfter   string
	searchExifDateBefore  string
	searchExifMinISO      int
	searchExifMaxISO      int
	searchExifMinAp       float64
	searchExifMaxAp       float64
	searchExifMinFocalLen float64
	searchExifMaxFocalLen float64
	searchExifLatMin      float64
	searchExifLatMax      float64
	searchExifLonMin      float64
	searchExifLonMax      float64
	searchXattrTags       string
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

var indexDirsCmd = &cobra.Command{
	Use:   "dirs",
	Short: "List configured index paths",
	RunE:  runIndexDirs,
}

var indexCheckCmd = &cobra.Command{
	Use:   "check <path>",
	Short: "Check whether a path is indexed",
	Args:  cobra.ExactArgs(1),
	RunE:  runIndexCheck,
}

var indexFilesCmd = &cobra.Command{
	Use:   "files [prefix]",
	Short: "List files in the metastore",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runIndexFiles,
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
	searchCmd.Flags().StringVar(&searchSort, "sort", "score", "sort by: score, mtime, size, filename, exif_date, iso, focal_length, aperture")
	searchCmd.Flags().BoolVar(&searchSortDesc, "desc", true, "sort descending")
	searchCmd.Flags().Int64Var(&searchMinSize, "min-size", 0, "minimum file size in bytes")
	searchCmd.Flags().Int64Var(&searchMaxSize, "max-size", 0, "maximum file size in bytes")
	searchCmd.Flags().BoolVar(&searchJSON, "json", false, "output results in JSON format")
	searchCmd.Flags().StringVar(&searchFolder, "folder", "", "filter by folder path (e.g., /home/user/Pictures)")
	searchCmd.Flags().StringVar(&searchExifMake, "exif-make", "", "filter by camera make (e.g., Canon)")
	searchCmd.Flags().StringVar(&searchExifModel, "exif-model", "", "filter by camera model (e.g., Canon EOS 5D)")
	searchCmd.Flags().StringVar(&searchExifDateAfter, "exif-date-after", "", "photos taken after date (RFC3339 or EXIF format)")
	searchCmd.Flags().StringVar(&searchExifDateBefore, "exif-date-before", "", "photos taken before date (RFC3339 or EXIF format)")
	searchCmd.Flags().IntVar(&searchExifMinISO, "exif-min-iso", 0, "minimum ISO value")
	searchCmd.Flags().IntVar(&searchExifMaxISO, "exif-max-iso", 0, "maximum ISO value")
	searchCmd.Flags().Float64Var(&searchExifMinAp, "exif-min-aperture", 0, "minimum aperture (f-number)")
	searchCmd.Flags().Float64Var(&searchExifMaxAp, "exif-max-aperture", 0, "maximum aperture (f-number)")
	searchCmd.Flags().Float64Var(&searchExifMinFocalLen, "exif-min-focal-len", 0, "minimum focal length in mm")
	searchCmd.Flags().Float64Var(&searchExifMaxFocalLen, "exif-max-focal-len", 0, "maximum focal length in mm")
	searchCmd.Flags().Float64Var(&searchExifLatMin, "exif-lat-min", 0, "minimum GPS latitude")
	searchCmd.Flags().Float64Var(&searchExifLatMax, "exif-lat-max", 0, "maximum GPS latitude")
	searchCmd.Flags().Float64Var(&searchExifLonMin, "exif-lon-min", 0, "minimum GPS longitude")
	searchCmd.Flags().Float64Var(&searchExifLonMax, "exif-lon-max", 0, "maximum GPS longitude")
	searchCmd.Flags().StringVar(&searchXattrTags, "xattr-tags", "", "tags in user.xdg.tags xattr")

	indexFilesCmd.Flags().IntVar(&filesLimit, "limit", 100, "maximum number of files to list")

	indexCmd.AddCommand(indexGenerateCmd)
	indexCmd.AddCommand(indexSyncCmd)
	indexCmd.AddCommand(indexStatusCmd)
	indexCmd.AddCommand(indexDirsCmd)
	indexCmd.AddCommand(indexCheckCmd)
	indexCmd.AddCommand(indexFilesCmd)

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

	count, err := idx.GetDocCount()
	if err != nil {
		return err
	}

	if count == 0 {
		log.Infof("index is empty, building initial index...")
		go func() {
			if err := idx.ReindexAll(); err != nil {
				log.Errorf("initial index build failed: %v", err)
			} else {
				log.Infof("initial index build complete")
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

	clientOpts := &client.SearchOptions{
		Query:           query,
		Limit:           limit,
		Field:           searchField,
		Extension:       searchExt,
		Fuzzy:           searchFuzzy,
		SortBy:          searchSort,
		SortDesc:        searchSortDesc,
		MinSize:         searchMinSize,
		MaxSize:         searchMaxSize,
		Folder:          searchFolder,
		ExifMake:        searchExifMake,
		ExifModel:       searchExifModel,
		ExifDateAfter:   searchExifDateAfter,
		ExifDateBefore:  searchExifDateBefore,
		ExifMinISO:      searchExifMinISO,
		ExifMaxISO:      searchExifMaxISO,
		ExifMinAperture: searchExifMinAp,
		ExifMaxAperture: searchExifMaxAp,
		ExifMinFocalLen: searchExifMinFocalLen,
		ExifMaxFocalLen: searchExifMaxFocalLen,
		ExifLatMin:      searchExifLatMin,
		ExifLatMax:      searchExifLatMax,
		ExifLonMin:      searchExifLonMin,
		ExifLonMax:      searchExifLonMax,
		XattrTags:       searchXattrTags,
	}

	result, err := client.SearchWithOptions(clientOpts)
	if err == nil {
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

	cfg := buildConfig()

	idx, err := indexer.New(cfg)
	if err != nil {
		return fmt.Errorf("server not running and cannot open index: %v", err)
	}
	defer idx.Close()

	count, err := idx.GetDocCount()
	if err != nil {
		return err
	}

	if count == 0 {
		return fmt.Errorf("index is empty - run 'dsearch index generate' or 'dsearch serve' first")
	}

	indexerOpts := &indexer.SearchOptions{
		Query:           query,
		Limit:           limit,
		Field:           searchField,
		Extension:       searchExt,
		Fuzzy:           searchFuzzy,
		SortBy:          searchSort,
		SortDesc:        searchSortDesc,
		MinSize:         searchMinSize,
		MaxSize:         searchMaxSize,
		Folder:          searchFolder,
		ExifMake:        searchExifMake,
		ExifModel:       searchExifModel,
		ExifDateAfter:   searchExifDateAfter,
		ExifDateBefore:  searchExifDateBefore,
		ExifMinISO:      searchExifMinISO,
		ExifMaxISO:      searchExifMaxISO,
		ExifMinAperture: searchExifMinAp,
		ExifMaxAperture: searchExifMaxAp,
		ExifMinFocalLen: searchExifMinFocalLen,
		ExifMaxFocalLen: searchExifMaxFocalLen,
		ExifLatMin:      searchExifLatMin,
		ExifLatMax:      searchExifLatMax,
		ExifLonMin:      searchExifLonMin,
		ExifLonMax:      searchExifLonMax,
		XattrTags:       searchXattrTags,
	}

	result, err = idx.SearchWithOptions(indexerOpts)
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
	if err == nil {
		log.Infof("%s", status)
		return nil
	}

	cfg := buildConfig()

	idx, err := indexer.New(cfg)
	if err != nil {
		return fmt.Errorf("server not running and cannot open index: %v", err)
	}
	defer idx.Close()

	log.Infof("starting full reindex...")
	if err := idx.ReindexAll(); err != nil {
		return err
	}

	return nil
}

func runIndexSync(cmd *cobra.Command, args []string) error {
	status, err := client.Sync()
	if err == nil {
		log.Infof("%s", status)
		return nil
	}

	cfg := buildConfig()

	idx, err := indexer.New(cfg)
	if err != nil {
		return fmt.Errorf("server not running and cannot open index: %v", err)
	}
	defer idx.Close()

	log.Infof("starting incremental sync...")
	if err := idx.SyncIncremental(); err != nil {
		return err
	}

	return nil
}

func runIndexStatus(cmd *cobra.Command, args []string) error {
	stats, err := client.Stats()
	if err == nil {
		log.Infof("Index Statistics:")
		log.Infof("  Total files: %v", stats["total_files"])
		log.Infof("  Total bytes: %v", stats["total_bytes"])
		log.Infof("  Last index: %v", stats["last_index_time"])
		log.Infof("  Duration: %v", stats["index_duration"])
		return nil
	}

	cfg := buildConfig()

	idx, err := indexer.New(cfg)
	if err != nil {
		return fmt.Errorf("server not running and cannot open index: %v", err)
	}
	defer idx.Close()

	indexStats := idx.Stats()

	log.Infof("Index Statistics:")
	log.Infof("  Total files: %d", indexStats.TotalFiles)
	log.Infof("  Total bytes: %d", indexStats.TotalBytes)
	log.Infof("  Last index: %v", indexStats.LastIndexTime.Format("2006-01-02 15:04:05"))
	log.Infof("  Duration: %s", indexStats.IndexDuration)

	return nil
}

func runIndexDirs(cmd *cobra.Command, args []string) error {
	cfg := buildConfig()
	for _, ip := range cfg.IndexPaths {
		depth := "unlimited"
		if ip.MaxDepth > 0 {
			depth = fmt.Sprintf("%d", ip.MaxDepth)
		}
		hidden := "included"
		if ip.ExcludeHidden {
			hidden = "excluded"
		}
		watch := "on"
		if !ip.ShouldWatch() {
			watch = "off"
		}
		exif := "off"
		if ip.ExtractExif {
			exif = "on"
		}
		fmt.Printf("%s (depth: %s, hidden: %s, watch: %s, exif: %s)\n",
			ip.Path, depth, hidden, watch, exif)
		if len(ip.ExcludeDirs) > 0 {
			fmt.Printf("  exclude: %s\n", strings.Join(ip.ExcludeDirs, ", "))
		}
	}
	return nil
}

func runIndexCheck(cmd *cobra.Command, args []string) error {
	path, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}

	cfg := buildConfig()
	idxPath := cfg.FindIndexPath(path)

	fmt.Printf("%s\n", path)

	if idxPath == nil {
		fmt.Printf("  index path: none\n")
		fmt.Printf("  config:     excluded (not under any configured index path)\n")
		return nil
	}
	fmt.Printf("  index path: %s\n", idxPath.Path)

	reason := cfg.ExclusionReason(path)
	if reason != "" {
		fmt.Printf("  config:     excluded (%s)\n", reason)
		return nil
	}
	fmt.Printf("  config:     included\n")

	info, statErr := os.Stat(path)
	if statErr == nil && info.IsDir() {
		depth := cfg.GetDepth(path)
		maxDepth := cfg.GetMaxDepth(path)
		if maxDepth > 0 {
			fmt.Printf("  depth:      %d (max: %d)\n", depth, maxDepth)
		} else {
			fmt.Printf("  depth:      %d (unlimited)\n", depth)
		}
		return nil
	}

	store, err := metastore.New(cfg.IndexPath)
	if err != nil {
		fmt.Printf("  indexed:    unknown (cannot open metastore: %v)\n", err)
		return nil
	}
	defer store.Close()

	meta, found, err := store.Get(path)
	if err != nil {
		fmt.Printf("  indexed:    unknown (metastore error: %v)\n", err)
		return nil
	}
	if found {
		fmt.Printf("  indexed:    yes (%s, %s)\n",
			meta.ModTime.Format("2006-01-02 15:04:05"), formatSize(meta.Size))
	} else {
		fmt.Printf("  indexed:    no\n")
	}
	return nil
}

func runIndexFiles(cmd *cobra.Command, args []string) error {
	prefix := ""
	if len(args) > 0 {
		prefix = args[0]
	}

	result, err := client.Files(prefix, filesLimit)
	if err == nil {
		for _, f := range result.Files {
			t, _ := time.Parse(time.RFC3339, f.ModTime)
			fmt.Printf("%s (%s, %s)\n", f.Path, t.Format("2006-01-02"), formatSize(f.Size))
		}
		if result.Total > filesLimit {
			fmt.Printf("(showing %d of %d files)\n", len(result.Files), result.Total)
		}
		return nil
	}

	cfg := buildConfig()
	idx, err := indexer.New(cfg)
	if err != nil {
		return fmt.Errorf("server not running and cannot open index: %v", err)
	}
	defer idx.Close()

	files, total, err := idx.ListFiles(prefix, filesLimit)
	if err != nil {
		return err
	}

	for _, f := range files {
		fmt.Printf("%s (%s, %s)\n", f.Path, f.ModTime.Format("2006-01-02"), formatSize(f.Size))
	}
	if total > filesLimit {
		fmt.Printf("(showing %d of %d files)\n", len(files), total)
	}
	return nil
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
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
