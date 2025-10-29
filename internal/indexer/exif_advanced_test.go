package indexer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AvengeMedia/danksearch/internal/config"
)

func TestSearchWithExifDateRange(t *testing.T) {
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
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := idx.Index(testFile); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	opts := &SearchOptions{
		Query:          "*",
		Limit:          10,
		ExifDateAfter:  "2024-01-01T00:00:00Z",
		ExifDateBefore: "2024-12-31T23:59:59Z",
	}

	_, err = idx.SearchWithOptions(opts)
	if err != nil {
		t.Errorf("SearchWithOptions() error = %v", err)
	}
}

func TestSearchWithISORange(t *testing.T) {
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
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := idx.Index(testFile); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	opts := &SearchOptions{
		Query:      "*",
		Limit:      10,
		ExifMinISO: 100,
		ExifMaxISO: 3200,
	}

	_, err = idx.SearchWithOptions(opts)
	if err != nil {
		t.Errorf("SearchWithOptions() error = %v", err)
	}
}

func TestSearchWithApertureRange(t *testing.T) {
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
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := idx.Index(testFile); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	opts := &SearchOptions{
		Query:           "*",
		Limit:           10,
		ExifMinAperture: 1.8,
		ExifMaxAperture: 5.6,
	}

	_, err = idx.SearchWithOptions(opts)
	if err != nil {
		t.Errorf("SearchWithOptions() error = %v", err)
	}
}

func TestSearchWithFocalLengthRange(t *testing.T) {
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
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := idx.Index(testFile); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	opts := &SearchOptions{
		Query:           "*",
		Limit:           10,
		ExifMinFocalLen: 24,
		ExifMaxFocalLen: 200,
	}

	_, err = idx.SearchWithOptions(opts)
	if err != nil {
		t.Errorf("SearchWithOptions() error = %v", err)
	}
}

func TestSearchWithGPSRange(t *testing.T) {
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
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := idx.Index(testFile); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	opts := &SearchOptions{
		Query:      "*",
		Limit:      10,
		ExifLatMin: 40.0,
		ExifLatMax: 41.0,
		ExifLonMin: -74.0,
		ExifLonMax: -73.0,
	}

	_, err = idx.SearchWithOptions(opts)
	if err != nil {
		t.Errorf("SearchWithOptions() error = %v", err)
	}
}

func TestSortByExifDate(t *testing.T) {
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
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := idx.Index(testFile); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	opts := &SearchOptions{
		Query:    "*",
		Limit:    10,
		SortBy:   "exif_date",
		SortDesc: true,
	}

	_, err = idx.SearchWithOptions(opts)
	if err != nil {
		t.Errorf("SearchWithOptions() error = %v", err)
	}
}

func TestSortByISO(t *testing.T) {
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
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := idx.Index(testFile); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	opts := &SearchOptions{
		Query:    "*",
		Limit:    10,
		SortBy:   "iso",
		SortDesc: false,
	}

	_, err = idx.SearchWithOptions(opts)
	if err != nil {
		t.Errorf("SearchWithOptions() error = %v", err)
	}
}

func TestSortByFocalLength(t *testing.T) {
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
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := idx.Index(testFile); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	opts := &SearchOptions{
		Query:    "*",
		Limit:    10,
		SortBy:   "focal_length",
		SortDesc: true,
	}

	_, err = idx.SearchWithOptions(opts)
	if err != nil {
		t.Errorf("SearchWithOptions() error = %v", err)
	}
}
