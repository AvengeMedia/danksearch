package indexer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AvengeMedia/danksearch/internal/config"
)

func TestNew(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Default()
	cfg.IndexPath = filepath.Join(tmpDir, "index")
	cfg.IndexPaths = []config.IndexPath{
		{
			Path:          tmpDir,
			MaxDepth:      10,
			ExcludeHidden: false,
			ExcludeDirs:   []string{},
		},
	}

	idx, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer idx.Close()

	if idx.index == nil {
		t.Error("index should not be nil")
	}

	if idx.config != cfg {
		t.Error("config should match")
	}
}

func TestIndexer_Index(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Default()
	cfg.IndexPath = filepath.Join(tmpDir, "index")
	cfg.IndexPaths = []config.IndexPath{
		{
			Path:          tmpDir,
			MaxDepth:      10,
			ExcludeHidden: false,
			ExcludeDirs:   []string{},
		},
	}
	cfg.BuildMaps()

	idx, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer idx.Close()

	testFile := filepath.Join(tmpDir, "test.txt")
	content := "hello world"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := idx.Index(testFile); err != nil {
		t.Errorf("Index() error = %v", err)
	}

	result, err := idx.Search("hello", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if result.Total == 0 {
		t.Error("expected at least one search result")
	}
}

func TestIndexer_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Default()
	cfg.IndexPath = filepath.Join(tmpDir, "index")
	cfg.IndexPaths = []config.IndexPath{
		{
			Path:          tmpDir,
			MaxDepth:      10,
			ExcludeHidden: false,
			ExcludeDirs:   []string{},
		},
	}
	cfg.BuildMaps()

	idx, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer idx.Close()

	testFile := filepath.Join(tmpDir, "test.txt")
	content := "hello world"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := idx.Index(testFile); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	if err := idx.Delete(testFile); err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	result, err := idx.Search("hello", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if result.Total != 0 {
		t.Errorf("expected zero results after delete, got %d", result.Total)
	}
}

func TestIndexer_Search(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Default()
	cfg.IndexPath = filepath.Join(tmpDir, "index")
	cfg.IndexPaths = []config.IndexPath{
		{
			Path:          tmpDir,
			MaxDepth:      10,
			ExcludeHidden: false,
			ExcludeDirs:   []string{},
		},
	}
	cfg.BuildMaps()

	idx, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer idx.Close()

	testFile1 := filepath.Join(tmpDir, "test1.txt")
	if err := os.WriteFile(testFile1, []byte("golang programming"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	testFile2 := filepath.Join(tmpDir, "test2.txt")
	if err := os.WriteFile(testFile2, []byte("python scripting"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	idx.Index(testFile1)
	idx.Index(testFile2)

	result, err := idx.Search("golang", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if result.Total == 0 {
		t.Error("expected at least one result for 'golang'")
	}

	result, err = idx.Search("python", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if result.Total == 0 {
		t.Error("expected at least one result for 'python'")
	}
}

func TestIndexer_Stats(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Default()
	cfg.IndexPath = filepath.Join(tmpDir, "index")

	idx, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer idx.Close()

	stats := idx.Stats()
	if stats == nil {
		t.Error("Stats() should not return nil")
	}
}

func TestReadDocument(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Default()
	cfg.IndexPath = filepath.Join(tmpDir, "index")
	cfg.IndexPaths = []config.IndexPath{
		{
			Path:          tmpDir,
			MaxDepth:      10,
			ExcludeHidden: false,
			ExcludeDirs:   []string{},
		},
	}
	cfg.BuildMaps()

	idx, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer idx.Close()

	testFile := filepath.Join(tmpDir, "test.go")
	content := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	doc, err := idx.readDocument(testFile, info)
	if err != nil {
		t.Fatalf("readDocument() error = %v", err)
	}

	if doc.Path != testFile {
		t.Errorf("Path = %v, want %v", doc.Path, testFile)
	}

	if doc.Title != "test.go" {
		t.Errorf("Title = %v, want test.go", doc.Title)
	}

	if doc.Body != content {
		t.Errorf("Body = %v, want %v", doc.Body, content)
	}

	if doc.Hash == "" {
		t.Error("Hash should not be empty")
	}
}
