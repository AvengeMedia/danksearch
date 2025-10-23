package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/AvengeMedia/danksearch/internal/server/models"
	bleve "github.com/blevesearch/bleve/v2"
)

func getSocketDir() string {
	if runtime := os.Getenv("XDG_RUNTIME_DIR"); runtime != "" {
		return runtime
	}

	if os.Getuid() == 0 {
		if _, err := os.Stat("/run"); err == nil {
			return "/run/danksearch"
		}
		return "/var/run/danksearch"
	}

	return os.TempDir()
}

func findRunningSocket() (string, error) {
	dir := getSocketDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("service not running")
	}

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "danksearch-") || !strings.HasSuffix(entry.Name(), ".sock") {
			continue
		}

		socketPath := filepath.Join(dir, entry.Name())
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			conn.Close()
			return socketPath, nil
		}
	}

	return "", fmt.Errorf("service not running")
}

func sendRequest(method string, params map[string]interface{}) (json.RawMessage, error) {
	socketPath, err := findRunningSocket()
	if err != nil {
		return nil, err
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("service not running")
	}
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		// Read and discard server info line
	}

	req := models.Request{
		ID:     1,
		Method: method,
		Params: params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	if _, err := conn.Write(append(data, '\n')); err != nil {
		return nil, err
	}

	if !scanner.Scan() {
		return nil, fmt.Errorf("no response from server")
	}

	var resp struct {
		ID     int             `json:"id,omitempty"`
		Result json.RawMessage `json:"result,omitempty"`
		Error  string          `json:"error,omitempty"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("%s", resp.Error)
	}

	return resp.Result, nil
}

type SearchOptions struct {
	Query         string
	Limit         int
	Field         string
	ContentType   string
	Extension     string
	Fuzzy         bool
	SortBy        string
	SortDesc      bool
	MinSize       int64
	MaxSize       int64
	ModifiedAfter string
	Facets        []string
}

func Search(query string, limit int) (*bleve.SearchResult, error) {
	return SearchWithOptions(&SearchOptions{
		Query: query,
		Limit: limit,
	})
}

func SearchWithOptions(opts *SearchOptions) (*bleve.SearchResult, error) {
	params := map[string]interface{}{
		"query": opts.Query,
		"limit": opts.Limit,
	}

	// Add optional parameters
	if opts.Field != "" {
		params["field"] = opts.Field
	}
	if opts.ContentType != "" {
		params["content_type"] = opts.ContentType
	}
	if opts.Extension != "" {
		params["extension"] = opts.Extension
	}
	if opts.Fuzzy {
		params["fuzzy"] = opts.Fuzzy
	}
	if opts.SortBy != "" {
		params["sort_by"] = opts.SortBy
	}
	if opts.SortDesc {
		params["sort_desc"] = opts.SortDesc
	}
	if opts.MinSize > 0 {
		params["min_size"] = opts.MinSize
	}
	if opts.MaxSize > 0 {
		params["max_size"] = opts.MaxSize
	}
	if opts.ModifiedAfter != "" {
		params["modified_after"] = opts.ModifiedAfter
	}
	if len(opts.Facets) > 0 {
		params["facets"] = opts.Facets
	}

	result, err := sendRequest("search", params)
	if err != nil {
		return nil, err
	}

	var searchResult bleve.SearchResult
	if err := json.Unmarshal(result, &searchResult); err != nil {
		return nil, err
	}

	return &searchResult, nil
}

func Reindex() (string, error) {
	result, err := sendRequest("reindex", nil)
	if err != nil {
		return "", err
	}

	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", err
	}

	return resp.Status, nil
}

func Sync() (string, error) {
	result, err := sendRequest("sync", nil)
	if err != nil {
		return "", err
	}

	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", err
	}

	return resp.Status, nil
}

func Stats() (map[string]interface{}, error) {
	result, err := sendRequest("stats", nil)
	if err != nil {
		return nil, err
	}

	var stats map[string]interface{}
	if err := json.Unmarshal(result, &stats); err != nil {
		return nil, err
	}

	return stats, nil
}

func WatchStatus() (string, error) {
	result, err := sendRequest("watch.status", nil)
	if err != nil {
		return "", err
	}

	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", err
	}

	return resp.Status, nil
}

func WatchStart() (string, error) {
	result, err := sendRequest("watch.start", nil)
	if err != nil {
		return "", err
	}

	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", err
	}

	return resp.Status, nil
}

func WatchStop() (string, error) {
	result, err := sendRequest("watch.stop", nil)
	if err != nil {
		return "", err
	}

	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", err
	}

	return resp.Status, nil
}
