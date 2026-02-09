package config

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.IndexPath == "" {
		t.Error("IndexPath should not be empty")
	}

	if cfg.ListenAddr != ":43654" {
		t.Errorf("ListenAddr = %v, want :43654", cfg.ListenAddr)
	}

	if cfg.MaxFileBytes != 2*1024*1024 {
		t.Errorf("MaxFileBytes = %v, want %v", cfg.MaxFileBytes, 2*1024*1024)
	}

	expectedWorkers := runtime.NumCPU() / 2
	if expectedWorkers < 1 {
		expectedWorkers = 1
	}
	if cfg.WorkerCount != expectedWorkers {
		t.Errorf("WorkerCount = %v, want %v", cfg.WorkerCount, expectedWorkers)
	}

	if len(cfg.IndexPaths) == 0 {
		t.Error("IndexPaths should not be empty")
	}

	if len(cfg.IndexPaths[0].ExcludeDirs) == 0 {
		t.Error("IndexPaths[0].ExcludeDirs should not be empty")
	}

	if len(cfg.TextExts) == 0 {
		t.Error("TextExts should not be empty")
	}

	if !cfg.IndexAllFiles {
		t.Error("IndexAllFiles should be true by default")
	}
}

func TestConfig_ShouldIndexFile(t *testing.T) {
	cfg := &Config{
		IndexAllFiles: true,
		IndexPaths: []IndexPath{
			{
				Path:          "/home/user",
				MaxDepth:      10,
				ExcludeHidden: true,
				ExcludeDirs:   []string{"node_modules", ".git"},
			},
		},
	}
	cfg.BuildMaps()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "valid go file",
			path:     "/home/user/project/main.go",
			expected: true,
		},
		{
			name:     "valid python file",
			path:     "/home/user/project/script.py",
			expected: true,
		},
		{
			name:     "image file",
			path:     "/home/user/project/photo.jpg",
			expected: true,
		},
		{
			name:     "binary file",
			path:     "/home/user/project/app.exe",
			expected: true,
		},
		{
			name:     "file without extension",
			path:     "/home/user/project/README",
			expected: true,
		},
		{
			name:     "hidden file",
			path:     "/home/user/project/.hidden",
			expected: false,
		},
		{
			name:     "file in excluded dir",
			path:     "/home/user/project/node_modules/package.json",
			expected: false,
		},
		{
			name:     "file in git dir",
			path:     "/home/user/project/.git/config",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.ShouldIndexFile(tt.path)
			if got != tt.expected {
				t.Errorf("ShouldIndexFile(%v) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestConfig_IsTextFile(t *testing.T) {
	cfg := Default()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "go file",
			path:     "/home/user/project/main.go",
			expected: true,
		},
		{
			name:     "python file",
			path:     "/home/user/project/script.py",
			expected: true,
		},
		{
			name:     "markdown file",
			path:     "/home/user/project/README.md",
			expected: true,
		},
		{
			name:     "image file",
			path:     "/home/user/project/photo.jpg",
			expected: false,
		},
		{
			name:     "binary file",
			path:     "/home/user/project/app.exe",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.IsTextFile(tt.path)
			if got != tt.expected {
				t.Errorf("IsTextFile(%v) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestConfig_RegexExcludeDirs(t *testing.T) {
	cfg := &Config{
		IndexAllFiles: true,
		IndexPaths: []IndexPath{
			{
				Path:        "/home/user",
				MaxDepth:    10,
				ExcludeDirs: []string{"node_modules", "/^build-/", `/^out-\d+$/`},
			},
		},
	}
	cfg.BuildMaps()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "exact match still works",
			path:     "/home/user/project/node_modules/pkg/index.js",
			expected: false,
		},
		{
			name:     "regex excludes build-release",
			path:     "/home/user/project/build-release/app.js",
			expected: false,
		},
		{
			name:     "regex excludes build-debug",
			path:     "/home/user/project/build-debug/app.js",
			expected: false,
		},
		{
			name:     "regex does not exclude mybuild",
			path:     "/home/user/project/mybuild/app.js",
			expected: true,
		},
		{
			name:     "regex excludes out-123",
			path:     "/home/user/project/out-123/app.js",
			expected: false,
		},
		{
			name:     "regex does not exclude output",
			path:     "/home/user/project/output/app.js",
			expected: true,
		},
		{
			name:     "non-excluded dir is indexed",
			path:     "/home/user/project/src/main.go",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.ShouldIndexFile(tt.path)
			if got != tt.expected {
				t.Errorf("ShouldIndexFile(%v) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestConfig_InvalidRegexSkipped(t *testing.T) {
	cfg := &Config{
		IndexAllFiles: true,
		IndexPaths: []IndexPath{
			{
				Path:        "/home/user",
				MaxDepth:    10,
				ExcludeDirs: []string{"/[unclosed/", "/^valid-/"},
			},
		},
	}
	cfg.BuildMaps()

	// Invalid regex is skipped, valid one still works
	if cfg.ShouldIndexFile("/home/user/project/valid-dir/file.go") {
		t.Error("valid regex /^valid-/ should exclude valid-dir")
	}

	// The invalid regex should not cause a panic or exclude everything
	if !cfg.ShouldIndexFile("/home/user/project/src/file.go") {
		t.Error("src should not be excluded")
	}
}

func TestConfig_BackwardsCompat(t *testing.T) {
	// Config with no regex entries should work exactly as before
	cfg := &Config{
		IndexAllFiles: true,
		IndexPaths: []IndexPath{
			{
				Path:          "/home/user",
				MaxDepth:      10,
				ExcludeHidden: true,
				ExcludeDirs:   []string{"node_modules", "dist", "build"},
			},
		},
	}
	cfg.BuildMaps()

	if cfg.ShouldIndexFile("/home/user/project/node_modules/pkg.json") {
		t.Error("node_modules should be excluded")
	}
	if cfg.ShouldIndexFile("/home/user/project/dist/bundle.js") {
		t.Error("dist should be excluded")
	}
	if !cfg.ShouldIndexFile("/home/user/project/src/main.go") {
		t.Error("src should not be excluded")
	}
}

func TestGetDefaultIndexPath(t *testing.T) {
	path := getDefaultIndexPath()

	if path == "" {
		t.Error("getDefaultIndexPath() should not return empty string")
	}

	if !filepath.IsAbs(path) {
		t.Errorf("getDefaultIndexPath() = %v, expected absolute path", path)
	}

	if !filepath.HasPrefix(filepath.Base(filepath.Dir(path)), "dsearch") && !filepath.HasPrefix(filepath.Base(filepath.Dir(path)), "danksearch") {
		t.Errorf("getDefaultIndexPath() = %v, expected to contain dsearch or danksearch", path)
	}
}
