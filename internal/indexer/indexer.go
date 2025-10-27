package indexer

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AvengeMedia/danksearch/internal/config"
	"github.com/AvengeMedia/danksearch/internal/errdefs"
	"github.com/AvengeMedia/danksearch/internal/log"
	bleve "github.com/blevesearch/bleve/v2"
	_ "github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	_ "github.com/blevesearch/bleve/v2/analysis/token/edgengram"
	_ "github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	_ "github.com/blevesearch/bleve/v2/analysis/token/ngram"
	_ "github.com/blevesearch/bleve/v2/analysis/tokenizer/single"
	"github.com/blevesearch/bleve/v2/mapping"
	query "github.com/blevesearch/bleve/v2/search/query"
)

type Document struct {
	Path           string    `json:"path"`
	Filename       string    `json:"filename"`
	FilenameSub    string    `json:"filename_sub"`
	FilenamePrefix string    `json:"filename_prefix"`
	Body           string    `json:"body"`
	ContentType    string    `json:"content_type"`
	ModTime        time.Time `json:"mtime"`
	Size           int64     `json:"size"`
	Hash           string    `json:"hash"`
}

type Indexer struct {
	index         bleve.Index
	config        *config.Config
	mu            sync.RWMutex
	indexComplete atomic.Bool
}

type SearchOptions struct {
	Query         string   `json:"query"`
	Limit         int      `json:"limit"`
	Field         string   `json:"field,omitempty"`          // filename, body, title, or empty for all
	ContentType   string   `json:"content_type,omitempty"`   // filter by mime type
	Extension     string   `json:"extension,omitempty"`      // filter by extension
	Fuzzy         bool     `json:"fuzzy,omitempty"`          // enable fuzzy matching
	SortBy        string   `json:"sort_by,omitempty"`        // score, mtime, size, filename
	SortDesc      bool     `json:"sort_desc,omitempty"`      // sort descending
	MinSize       int64    `json:"min_size,omitempty"`       // minimum file size
	MaxSize       int64    `json:"max_size,omitempty"`       // maximum file size
	ModifiedAfter string   `json:"modified_after,omitempty"` // RFC3339 timestamp
	Facets        []string `json:"facets,omitempty"`         // facet fields: content_type, extension
}

func New(cfg *config.Config) (*Indexer, error) {
	idx, err := openOrCreateIndex(cfg.IndexPath)
	if err != nil {
		return nil, errdefs.NewCustomError(errdefs.ErrTypeIndexingFailed, "failed to open index", err)
	}

	i := &Indexer{
		index:  idx,
		config: cfg,
	}

	count, err := idx.DocCount()
	if err == nil && count > 0 {
		i.indexComplete.Store(true)
		log.Infof("loaded existing index with %d documents", count)
	}

	return i, nil
}

func openOrCreateIndex(path string) (bleve.Index, error) {
	idx, err := bleve.Open(path)
	if err == bleve.ErrorIndexPathDoesNotExist {
		mapping := buildIndexMapping()
		idx, err = bleve.NewUsing(path, mapping, "scorch", "scorch", getIndexConfig())
		if err != nil {
			return nil, err
		}
		log.Infof("created new index at %s", path)
		return idx, nil
	}
	if err != nil {
		return nil, err
	}
	log.Infof("opened existing index at %s", path)
	return idx, nil
}

func getIndexConfig() map[string]interface{} {
	return map[string]interface{}{
		"create_if_missing": true,
		"error_if_exists":   false,
		"unsafe_batch":      false,
		"store":             getStoreConfig(),
	}
}

func getStoreConfig() map[string]interface{} {
	return map[string]interface{}{
		"mmap":              false,
		"metrics":           false,
		"create_if_missing": true,
		"error_if_exists":   false,
	}
}

func buildIndexMapping() mapping.IndexMapping {
	m := bleve.NewIndexMapping()

	err := m.AddCustomTokenFilter("ngram_2_15", map[string]interface{}{
		"type": "ngram",
		"min":  float64(2),
		"max":  float64(15),
	})
	if err != nil {
		panic(err)
	}

	err = m.AddCustomTokenFilter("edge_ngram_2_30", map[string]interface{}{
		"type": "edge_ngram",
		"min":  float64(2),
		"max":  float64(30),
	})
	if err != nil {
		panic(err)
	}

	err = m.AddCustomAnalyzer("filename_ngram", map[string]interface{}{
		"type":      "custom",
		"tokenizer": "single",
		"token_filters": []string{
			"to_lower",
			"ngram_2_15",
		},
	})
	if err != nil {
		panic(err)
	}

	err = m.AddCustomAnalyzer("filename_edge", map[string]interface{}{
		"type":      "custom",
		"tokenizer": "single",
		"token_filters": []string{
			"to_lower",
			"edge_ngram_2_30",
		},
	})
	if err != nil {
		panic(err)
	}

	docMapping := bleve.NewDocumentMapping()

	pathField := bleve.NewTextFieldMapping()
	pathField.Analyzer = "keyword"
	pathField.Store = true
	docMapping.AddFieldMappingsAt("path", pathField)

	filenameField := bleve.NewTextFieldMapping()
	filenameField.Store = true
	filenameField.Analyzer = "keyword"
	docMapping.AddFieldMappingsAt("filename", filenameField)

	filenameSubField := bleve.NewTextFieldMapping()
	filenameSubField.Store = false
	filenameSubField.Analyzer = "filename_ngram"
	docMapping.AddFieldMappingsAt("filename_sub", filenameSubField)

	filenamePrefixField := bleve.NewTextFieldMapping()
	filenamePrefixField.Store = false
	filenamePrefixField.Analyzer = "filename_edge"
	docMapping.AddFieldMappingsAt("filename_prefix", filenamePrefixField)

	bodyField := bleve.NewTextFieldMapping()
	bodyField.Store = false
	bodyField.IncludeTermVectors = false
	docMapping.AddFieldMappingsAt("body", bodyField)

	contentTypeField := bleve.NewTextFieldMapping()
	contentTypeField.Store = true
	docMapping.AddFieldMappingsAt("content_type", contentTypeField)

	mtimeField := bleve.NewDateTimeFieldMapping()
	mtimeField.Store = true
	docMapping.AddFieldMappingsAt("mtime", mtimeField)

	sizeField := bleve.NewNumericFieldMapping()
	sizeField.Store = true
	docMapping.AddFieldMappingsAt("size", sizeField)

	hashField := bleve.NewTextFieldMapping()
	hashField.Store = true
	hashField.Analyzer = "keyword"
	docMapping.AddFieldMappingsAt("hash", hashField)

	m.DefaultMapping = docMapping
	return m
}

func (i *Indexer) Index(path string) error {
	if !i.config.ShouldIndexFile(path) {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsPermission(err) {
			return errdefs.NewCustomError(errdefs.ErrTypeFileAccessDenied, path, err)
		}
		return err
	}

	if info.IsDir() {
		return nil
	}

	// Read document without holding lock (file I/O can be slow)
	doc, err := i.readDocument(path, info)
	if err != nil {
		return err
	}

	// Only lock when writing to index
	i.mu.Lock()
	err = i.index.Index(path, doc)
	i.mu.Unlock()

	if err != nil {
		return errdefs.NewCustomError(errdefs.ErrTypeIndexingFailed, path, err)
	}

	// Mark index as complete after first successful index
	i.indexComplete.Store(true)

	log.Debugf("indexed %s", path)
	return nil
}

func (i *Indexer) readDocument(path string, info os.FileInfo) (*Document, error) {
	filename := filepath.Base(path)
	ext := filepath.Ext(path)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	doc := &Document{
		Path:           path,
		Filename:       filename,
		FilenameSub:    filename,
		FilenamePrefix: filename,
		ContentType:    contentType,
		ModTime:        info.ModTime(),
		Size:           info.Size(),
	}

	if i.config.IsTextFile(path) {
		f, err := os.Open(path)
		if err != nil {
			return doc, nil
		}
		defer f.Close()

		limited := io.LimitReader(f, i.config.MaxFileBytes)
		content, err := io.ReadAll(limited)
		if err != nil {
			return doc, nil
		}

		hash := sha256.Sum256(content)
		doc.Body = string(content)
		doc.Hash = hex.EncodeToString(hash[:])
	}

	return doc, nil
}

func (i *Indexer) Delete(path string) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if err := i.index.Delete(path); err != nil {
		return errdefs.NewCustomError(errdefs.ErrTypeIndexingFailed, "delete failed", err)
	}

	log.Debugf("deleted %s from index", path)
	return nil
}

func (i *Indexer) Search(query string, limit int) (*bleve.SearchResult, error) {
	return i.SearchWithOptions(&SearchOptions{
		Query: query,
		Limit: limit,
	})
}

func (i *Indexer) SearchWithOptions(opts *SearchOptions) (*bleve.SearchResult, error) {
	if !i.indexComplete.Load() {
		return nil, errdefs.NewCustomError(errdefs.ErrTypeSearchFailed, "index not ready", nil)
	}

	i.mu.RLock()
	defer i.mu.RUnlock()

	if opts.Limit <= 0 {
		opts.Limit = 10
	}

	// Build the main query
	var mainQuery query.Query

	if opts.Query == "*" {
		mainQuery = bleve.NewMatchAllQuery()
	} else if opts.Field != "" {
		mainQuery = i.buildFieldQuery(opts.Query, opts.Field, opts.Fuzzy)
	} else {
		filenameQuery := i.buildFilenameQuery(opts.Query, 20.0, 10.0)
		bodyQuery := bleve.NewMatchQuery(opts.Query)
		bodyQuery.SetField("body")
		bodyQuery.SetBoost(1.0)

		mainQuery = bleve.NewDisjunctionQuery(filenameQuery, bodyQuery)
	}

	// Build filters
	filters := []query.Query{}

	if opts.ContentType != "" {
		ctQuery := bleve.NewMatchQuery(opts.ContentType)
		ctQuery.SetField("content_type")
		filters = append(filters, ctQuery)
	}

	if opts.Extension != "" {
		extPattern := "*" + strings.ToLower(opts.Extension)
		extQuery := bleve.NewWildcardQuery(extPattern)
		extQuery.SetField("filename")
		filters = append(filters, extQuery)
	}

	if opts.MinSize > 0 {
		minSizeFloat := float64(opts.MinSize)
		sizeQuery := bleve.NewNumericRangeInclusiveQuery(&minSizeFloat, nil, nil, nil)
		sizeQuery.SetField("size")
		filters = append(filters, sizeQuery)
	}

	if opts.MaxSize > 0 {
		maxSizeFloat := float64(opts.MaxSize)
		sizeQuery := bleve.NewNumericRangeInclusiveQuery(nil, &maxSizeFloat, nil, nil)
		sizeQuery.SetField("size")
		filters = append(filters, sizeQuery)
	}

	if opts.ModifiedAfter != "" {
		if t, err := time.Parse(time.RFC3339, opts.ModifiedAfter); err == nil {
			dateQuery := bleve.NewDateRangeInclusiveQuery(t, time.Time{}, nil, nil)
			dateQuery.SetField("mtime")
			filters = append(filters, dateQuery)
		}
	}

	// Combine main query with filters
	var finalQuery query.Query
	if len(filters) > 0 {
		conjunctQueries := append([]query.Query{mainQuery}, filters...)
		finalQuery = bleve.NewConjunctionQuery(conjunctQueries...)
	} else {
		finalQuery = mainQuery
	}

	// Build search request
	req := bleve.NewSearchRequest(finalQuery)
	req.Size = opts.Limit
	req.Highlight = bleve.NewHighlight()

	// Add facets
	for _, facet := range opts.Facets {
		req.AddFacet(facet, bleve.NewFacetRequest(facet, 10))
	}

	// Set sorting
	sortBy := opts.SortBy
	if sortBy == "" {
		sortBy = "score"
	}

	switch sortBy {
	case "mtime":
		if opts.SortDesc {
			req.SortBy([]string{"-mtime"})
		} else {
			req.SortBy([]string{"mtime"})
		}
	case "size":
		if opts.SortDesc {
			req.SortBy([]string{"-size"})
		} else {
			req.SortBy([]string{"size"})
		}
	case "filename":
		if opts.SortDesc {
			req.SortBy([]string{"-filename"})
		} else {
			req.SortBy([]string{"filename"})
		}
	default: // score
		req.SortBy([]string{"-_score"})
	}

	result, err := i.index.Search(req)
	if err != nil {
		return nil, errdefs.NewCustomError(errdefs.ErrTypeSearchFailed, opts.Query, err)
	}

	return result, nil
}

func (i *Indexer) buildFilenameQuery(queryStr string, boostPrefix, boostContains float64) query.Query {
	q := strings.TrimSpace(queryStr)
	if q == "" {
		return bleve.NewMatchNoneQuery()
	}

	if strings.Contains(q, "*") || strings.Contains(q, "?") {
		wildcardQuery := bleve.NewWildcardQuery(strings.ToLower(q))
		wildcardQuery.SetField("filename")
		return wildcardQuery
	}

	disj := bleve.NewDisjunctionQuery()

	prefixQuery := bleve.NewPrefixQuery(strings.ToLower(q))
	prefixQuery.SetField("filename_prefix")
	prefixQuery.SetBoost(boostPrefix)
	disj.AddQuery(prefixQuery)

	if len(q) >= 2 {
		matchQuery := bleve.NewMatchQuery(q)
		matchQuery.SetField("filename_sub")
		matchQuery.SetBoost(boostContains)
		disj.AddQuery(matchQuery)
	}

	if len(disj.Disjuncts) == 1 {
		return disj.Disjuncts[0]
	}
	return disj
}

func (i *Indexer) buildFieldQuery(queryStr, field string, fuzzy bool) query.Query {
	if field == "filename" {
		return i.buildFilenameQuery(queryStr, 2.0, 1.0)
	}

	if field == "body" {
		if fuzzy {
			q := bleve.NewFuzzyQuery(queryStr)
			q.SetField("body")
			return q
		}
		q := bleve.NewMatchQuery(queryStr)
		q.SetField("body")
		return q
	}

	q := bleve.NewMatchQuery(queryStr)
	q.SetField(field)
	return q
}

func (i *Indexer) ReindexAll() error {
	start := time.Now()

	var totalFiles int64
	var totalBytes int64
	var mu sync.Mutex
	semaphore := make(chan struct{}, i.config.WorkerCount)
	var wg sync.WaitGroup

	for _, idxPath := range i.config.IndexPaths {
		log.Infof("indexing %s (max_depth: %d)", idxPath.Path, idxPath.MaxDepth)

		err := filepath.Walk(idxPath.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsPermission(err) {
					log.Debugf("permission denied: %s", path)
					return nil
				}
				return err
			}

			if info.IsDir() {
				depth := i.config.GetDepth(path)
				maxDepth := i.config.GetMaxDepth(path)
				if maxDepth > 0 && depth >= maxDepth {
					return filepath.SkipDir
				}

				if !i.config.ShouldIndexDir(path) {
					return filepath.SkipDir
				}
				return nil
			}

			if !i.config.ShouldIndexFile(path) {
				return nil
			}

			wg.Add(1)
			go func(p string, inf os.FileInfo) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				if err := i.Index(p); err != nil {
					log.Debugf("failed to index %s: %v", p, err)
					return
				}

				mu.Lock()
				atomic.AddInt64(&totalFiles, 1)
				atomic.AddInt64(&totalBytes, inf.Size())
				mu.Unlock()
			}(path, info)

			return nil
		})

		if err != nil {
			return errdefs.NewCustomError(errdefs.ErrTypeIndexingFailed, "walk failed", err)
		}
	}

	wg.Wait()

	duration := time.Since(start)
	i.indexComplete.Store(true)

	if err := i.saveStatsDocument(int(totalFiles), totalBytes, duration); err != nil {
		log.Warnf("failed to save stats: %v", err)
	}

	log.Infof("reindex complete: %d files, %d bytes, took %s", totalFiles, totalBytes, duration)
	return nil
}

func (i *Indexer) Stats() *config.IndexStats {
	stats, err := i.calculateStats()
	if err != nil {
		log.Warnf("failed to calculate stats: %v", err)
		return &config.IndexStats{}
	}
	return stats
}

func (i *Indexer) calculateStats() (*config.IndexStats, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	count, err := i.index.DocCount()
	if err != nil {
		return nil, err
	}

	statsDoc, err := i.loadStatsDocument()
	if err != nil || statsDoc == nil {
		return &config.IndexStats{
			TotalFiles:    int(count),
			TotalBytes:    0,
			LastIndexTime: time.Time{},
			IndexDuration: "stats unavailable",
		}, nil
	}

	return statsDoc, nil
}

type statsMetadata struct {
	TotalFiles    int       `json:"total_files"`
	TotalBytes    int64     `json:"total_bytes"`
	LastIndexTime time.Time `json:"last_index_time"`
	IndexDuration string    `json:"index_duration"`
}

func (i *Indexer) loadStatsDocument() (*config.IndexStats, error) {
	req := bleve.NewSearchRequest(bleve.NewDocIDQuery([]string{"__stats__"}))
	req.Fields = []string{"total_files", "total_bytes", "last_index_time", "index_duration"}

	result, err := i.index.Search(req)
	if err != nil || len(result.Hits) == 0 {
		return nil, err
	}

	hit := result.Hits[0]
	stats := &config.IndexStats{}

	if totalFiles, ok := hit.Fields["total_files"].(float64); ok {
		stats.TotalFiles = int(totalFiles)
	}
	if totalBytes, ok := hit.Fields["total_bytes"].(float64); ok {
		stats.TotalBytes = int64(totalBytes)
	}
	if duration, ok := hit.Fields["index_duration"].(string); ok {
		stats.IndexDuration = duration
	}
	if lastIndexStr, ok := hit.Fields["last_index_time"].(string); ok {
		stats.LastIndexTime, _ = time.Parse(time.RFC3339, lastIndexStr)
	}

	return stats, nil
}

func (i *Indexer) saveStatsDocument(totalFiles int, totalBytes int64, duration time.Duration) error {
	stats := statsMetadata{
		TotalFiles:    totalFiles,
		TotalBytes:    totalBytes,
		LastIndexTime: time.Now(),
		IndexDuration: duration.String(),
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	return i.index.Index("__stats__", stats)
}

func (i *Indexer) SyncIncremental() error {
	start := time.Now()
	log.Infof("starting incremental sync")

	var added, updated, deleted, unchanged int64
	var totalBytes int64
	var pathsMu sync.Mutex // Protects indexedPaths map
	var statsMu sync.Mutex // Protects totalBytes
	semaphore := make(chan struct{}, i.config.WorkerCount)
	var wg sync.WaitGroup

	// Get all indexed paths
	indexedPaths := make(map[string]*Document)
	count, err := i.index.DocCount()
	if err != nil {
		return err
	}

	req := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
	req.Size = int(count)
	req.Fields = []string{"path", "mtime", "size", "hash"}

	result, err := i.index.Search(req)
	if err != nil {
		return err
	}

	for _, hit := range result.Hits {
		doc := &Document{Path: hit.ID}
		if mtimeStr, ok := hit.Fields["mtime"].(string); ok {
			doc.ModTime, _ = time.Parse(time.RFC3339, mtimeStr)
		}
		if size, ok := hit.Fields["size"].(float64); ok {
			doc.Size = int64(size)
		}
		if hash, ok := hit.Fields["hash"].(string); ok {
			doc.Hash = hash
		}
		indexedPaths[hit.ID] = doc
	}

	// Scan filesystem and compare
	for _, idxPath := range i.config.IndexPaths {
		err := filepath.Walk(idxPath.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsPermission(err) {
					return nil
				}
				return err
			}

			if info.IsDir() {
				depth := i.config.GetDepth(path)
				maxDepth := i.config.GetMaxDepth(path)
				if maxDepth > 0 && depth >= maxDepth {
					return filepath.SkipDir
				}
				if !i.config.ShouldIndexDir(path) {
					return filepath.SkipDir
				}
				return nil
			}

			if !i.config.ShouldIndexFile(path) {
				return nil
			}

			wg.Add(1)
			go func(p string, inf os.FileInfo) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				// Check existing doc with lock
				pathsMu.Lock()
				existingDoc, exists := indexedPaths[p]
				delete(indexedPaths, p) // Mark as seen
				pathsMu.Unlock()

				// Check if we need to update
				needsUpdate := false
				if !exists {
					needsUpdate = true
					atomic.AddInt64(&added, 1)
				} else if !existingDoc.ModTime.Equal(inf.ModTime()) {
					needsUpdate = true
					atomic.AddInt64(&updated, 1)
				} else {
					atomic.AddInt64(&unchanged, 1)
					return
				}

				if needsUpdate {
					if err := i.Index(p); err != nil {
						log.Debugf("failed to index %s: %v", p, err)
						return
					}
					statsMu.Lock()
					totalBytes += inf.Size()
					statsMu.Unlock()
				}
			}(path, info)

			return nil
		})

		if err != nil {
			return errdefs.NewCustomError(errdefs.ErrTypeIndexingFailed, "walk failed", err)
		}
	}

	wg.Wait()

	// Delete files that no longer exist
	for path := range indexedPaths {
		if err := i.Delete(path); err != nil {
			log.Debugf("failed to delete %s: %v", path, err)
		} else {
			atomic.AddInt64(&deleted, 1)
		}
	}

	duration := time.Since(start)
	i.indexComplete.Store(true)

	count, _ = i.index.DocCount()
	if err := i.saveStatsDocument(int(count), totalBytes, duration); err != nil {
		log.Warnf("failed to save stats: %v", err)
	}

	log.Infof("incremental sync complete: +%d new, ~%d updated, -%d deleted, =%d unchanged, took %s",
		added, updated, deleted, unchanged, duration)

	return nil
}

func (i *Indexer) ShouldReindex(intervalHours int) bool {
	if intervalHours <= 0 {
		return false
	}

	stats, err := i.loadStatsDocument()
	if err != nil || stats == nil {
		return true
	}

	if stats.LastIndexTime.IsZero() {
		return true
	}

	interval := time.Duration(intervalHours) * time.Hour
	return time.Since(stats.LastIndexTime) >= interval
}

func (i *Indexer) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.index.Close()
}
