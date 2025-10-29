package indexer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AvengeMedia/danksearch/internal/config"
)

func TestExifExtraction(t *testing.T) {
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

	testJpegData := []byte{
		0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x18, 0x45, 0x78, 0x69, 0x66, 0x00, 0x00,
		0x4D, 0x4D, 0x00, 0x2A, 0x00, 0x00, 0x00, 0x08, 0x00, 0x00, 0x00, 0x00, 0xFF, 0xD9,
	}
	testFile := filepath.Join(tmpDir, "test.jpg")
	if err := os.WriteFile(testFile, testJpegData, 0644); err != nil {
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

	if doc.ContentType != "image/jpeg" {
		t.Errorf("ContentType = %v, want image/jpeg", doc.ContentType)
	}
}

func TestIsImageFile(t *testing.T) {
	tests := []struct {
		contentType string
		want        bool
	}{
		{"image/jpeg", true},
		{"image/png", true},
		{"image/gif", true},
		{"text/plain", false},
		{"application/pdf", false},
	}

	for _, tt := range tests {
		got := isImageFile(tt.contentType)
		if got != tt.want {
			t.Errorf("isImageFile(%v) = %v, want %v", tt.contentType, got, tt.want)
		}
	}
}

func TestSearchWithExifFilters(t *testing.T) {
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
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := idx.Index(testFile); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	opts := &SearchOptions{
		Query:     "*",
		Limit:     10,
		ExifMake:  "Canon",
		ExifModel: "EOS 5D",
	}

	_, err = idx.SearchWithOptions(opts)
	if err != nil {
		t.Errorf("SearchWithOptions() error = %v", err)
	}
}

func TestSearchWithFolderFilter(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

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

	testFile1 := filepath.Join(tmpDir, "root.txt")
	if err := os.WriteFile(testFile1, []byte("root file"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	testFile2 := filepath.Join(subDir, "sub.txt")
	if err := os.WriteFile(testFile2, []byte("sub file"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := idx.Index(testFile1); err != nil {
		t.Fatalf("Index() error = %v", err)
	}
	if err := idx.Index(testFile2); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	opts := &SearchOptions{
		Query:  "*",
		Limit:  10,
		Folder: subDir,
	}

	result, err := idx.SearchWithOptions(opts)
	if err != nil {
		t.Fatalf("SearchWithOptions() error = %v", err)
	}

	if result.Total == 0 {
		t.Error("expected at least one result in subdir")
	}

	found := false
	for _, hit := range result.Hits {
		if hit.ID == testFile2 {
			found = true
		}
		if hit.ID == testFile1 {
			t.Error("should not find root.txt when filtering by subdir")
		}
	}

	if !found {
		t.Error("expected to find sub.txt")
	}
}
