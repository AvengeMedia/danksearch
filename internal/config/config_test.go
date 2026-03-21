package config

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ConfigSuite struct {
	suite.Suite
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}

func (s *ConfigSuite) TestDefault() {
	cfg := Default()

	s.NotEmpty(cfg.IndexPath)
	s.Equal("127.0.0.1:43654", cfg.ListenAddr)
	s.Equal(int64(2*1024*1024), cfg.MaxFileBytes)

	expectedWorkers := runtime.NumCPU() / 2
	if expectedWorkers < 1 {
		expectedWorkers = 1
	}
	s.Equal(expectedWorkers, cfg.WorkerCount)
	s.NotEmpty(cfg.IndexPaths)
	s.NotEmpty(cfg.IndexPaths[0].ExcludeDirs)
	s.NotEmpty(cfg.TextExts)
	s.True(cfg.IndexAllFiles)
}

func (s *ConfigSuite) TestShouldIndexFile() {
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
		{"valid go file", "/home/user/project/main.go", true},
		{"valid python file", "/home/user/project/script.py", true},
		{"image file", "/home/user/project/photo.jpg", true},
		{"binary file", "/home/user/project/app.exe", true},
		{"file without extension", "/home/user/project/README", true},
		{"hidden file", "/home/user/project/.hidden", false},
		{"file in excluded dir", "/home/user/project/node_modules/package.json", false},
		{"file in git dir", "/home/user/project/.git/config", false},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.expected, cfg.ShouldIndexFile(tt.path))
		})
	}
}

func (s *ConfigSuite) TestIsTextFile() {
	cfg := Default()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"go file", "/home/user/project/main.go", true},
		{"python file", "/home/user/project/script.py", true},
		{"markdown file", "/home/user/project/README.md", true},
		{"image file", "/home/user/project/photo.jpg", false},
		{"binary file", "/home/user/project/app.exe", false},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.expected, cfg.IsTextFile(tt.path))
		})
	}
}

func (s *ConfigSuite) TestRegexExcludeDirs() {
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
		{"exact match still works", "/home/user/project/node_modules/pkg/index.js", false},
		{"regex excludes build-release", "/home/user/project/build-release/app.js", false},
		{"regex excludes build-debug", "/home/user/project/build-debug/app.js", false},
		{"regex does not exclude mybuild", "/home/user/project/mybuild/app.js", true},
		{"regex excludes out-123", "/home/user/project/out-123/app.js", false},
		{"regex does not exclude output", "/home/user/project/output/app.js", true},
		{"non-excluded dir is indexed", "/home/user/project/src/main.go", true},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.expected, cfg.ShouldIndexFile(tt.path))
		})
	}
}

func (s *ConfigSuite) TestInvalidRegexSkipped() {
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

	s.False(cfg.ShouldIndexFile("/home/user/project/valid-dir/file.go"))
	s.True(cfg.ShouldIndexFile("/home/user/project/src/file.go"))
}

func (s *ConfigSuite) TestBackwardsCompat() {
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

	s.False(cfg.ShouldIndexFile("/home/user/project/node_modules/pkg.json"))
	s.False(cfg.ShouldIndexFile("/home/user/project/dist/bundle.js"))
	s.True(cfg.ShouldIndexFile("/home/user/project/src/main.go"))
}

func (s *ConfigSuite) TestGetDefaultIndexPath() {
	path := getDefaultIndexPath()

	s.NotEmpty(path)
	s.True(filepath.IsAbs(path))
}
