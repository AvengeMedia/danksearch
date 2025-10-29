package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/AvengeMedia/danksearch/internal/log"
	"github.com/BurntSushi/toml"
)

type IndexPath struct {
	Path          string   `toml:"path"`
	MaxDepth      int      `toml:"max_depth"`
	ExcludeHidden bool     `toml:"exclude_hidden"`
	ExcludeDirs   []string `toml:"exclude_dirs"`
	ExtractExif   bool     `toml:"extract_exif"`

	excludeDirsMap map[string]bool
}

type Config struct {
	IndexPath            string      `toml:"index_path"`
	ListenAddr           string      `toml:"listen_addr"`
	MaxFileBytes         int64       `toml:"max_file_bytes"`
	WorkerCount          int         `toml:"worker_count"`
	IndexPaths           []IndexPath `toml:"index_paths"`
	TextExts             []string    `toml:"text_extensions"`
	IndexAllFiles        bool        `toml:"index_all_files"`
	AutoReindex          bool        `toml:"auto_reindex"`
	ReindexIntervalHours int         `toml:"reindex_interval_hours"`

	RootDir       string   `toml:"root_dir,omitempty"`
	MaxDepth      int      `toml:"max_depth,omitempty"`
	ExcludeHidden bool     `toml:"exclude_hidden,omitempty"`
	ExcludeDirs   []string `toml:"exclude_dirs,omitempty"`

	textExtsMap map[string]bool
}

func Default() *Config {
	home, _ := os.UserHomeDir()

	defaultExcludeDirs := []string{
		// JavaScript/Node.js
		"node_modules",
		"bower_components",
		".npm",
		".yarn",

		// Python
		"site-packages",
		"__pycache__",
		".venv",
		"venv",
		".tox",
		".pytest_cache",
		".eggs",

		// Build outputs
		"dist",
		"build",
		"out",
		"bin",
		"obj",

		// Rust
		"target",

		// Go
		"vendor",

		// Java/JVM
		".gradle",
		".m2",

		// Ruby
		"bundle",

		// Cache directories
		".cache",
		".parcel-cache",
		".next",
		".nuxt",
		".serverless",

		// OS specific
		"Library",
		".Trash-1000",

		// Databases
		".postgresql",
		".postgres",
		".mysql",
		".mongodb",
		".redis",
		"pgdata",
		"pg_data",

		// Language package manager caches
		"go",        // ~/go/pkg/mod - Go module cache
		".cargo",    // Rust cargo registry
		".pyenv",    // Python version manager
		".rbenv",    // Ruby version manager
		".nvm",      // Node version manager
		".rustup",   // Rust toolchain manager
		".composer", // PHP composer cache
		".gem",      // Ruby gems

		// IDE/Editor (though these are often hidden)
		".idea",
		".vscode",
	}

	workerCount := runtime.NumCPU() / 2
	if workerCount < 1 {
		workerCount = 1
	}

	cfg := &Config{
		IndexPath:            getDefaultIndexPath(),
		ListenAddr:           ":43654",
		MaxFileBytes:         2 * 1024 * 1024,
		WorkerCount:          workerCount,
		IndexAllFiles:        true,
		AutoReindex:          false,
		ReindexIntervalHours: 24,
		IndexPaths: []IndexPath{
			{
				Path:          home,
				MaxDepth:      6,
				ExcludeHidden: true,
				ExcludeDirs:   defaultExcludeDirs,
				ExtractExif:   true,
			},
		},
		TextExts: []string{
			".txt", ".md", ".go", ".py", ".js", ".ts",
			".jsx", ".tsx", ".json", ".yaml", ".yml",
			".toml", ".html", ".css", ".rs", ".c",
			".cpp", ".h", ".java", ".rb", ".php", ".sh",
		},
	}

	cfg.BuildMaps()
	return cfg
}

func Load(path string) (*Config, error) {
	cfg := Default()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := cfg.Save(path); err != nil {
			log.Warnf("failed to create default config at %s: %v", path, err)
		} else {
			log.Infof("created default config at %s", path)
		}
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, err
	}

	if cfg.RootDir != "" && len(cfg.IndexPaths) == 0 {
		cfg.IndexPaths = []IndexPath{
			{
				Path:          cfg.RootDir,
				MaxDepth:      cfg.MaxDepth,
				ExcludeHidden: cfg.ExcludeHidden,
				ExcludeDirs:   cfg.ExcludeDirs,
			},
		}
	}

	cfg.BuildMaps()
	return cfg, nil
}

func (c *Config) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString("# DankSearch Configuration\n")
	f.WriteString("# See https://github.com/AvengeMedia/danksearch for documentation\n\n")

	return toml.NewEncoder(f).Encode(c)
}

func (c *Config) BuildMaps() {
	for i := range c.IndexPaths {
		c.IndexPaths[i].excludeDirsMap = make(map[string]bool, len(c.IndexPaths[i].ExcludeDirs))
		for _, dir := range c.IndexPaths[i].ExcludeDirs {
			c.IndexPaths[i].excludeDirsMap[dir] = true
		}
	}

	c.textExtsMap = make(map[string]bool, len(c.TextExts))
	for _, ext := range c.TextExts {
		c.textExtsMap[ext] = true
	}
}

func getDefaultIndexPath() string {
	var base string
	if runtime.GOOS == "windows" {
		base = os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
	} else {
		base = os.Getenv("XDG_CACHE_HOME")
		if base == "" {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, ".cache")
		}
	}
	return filepath.Join(base, "danksearch", "index")
}

func GetDefaultConfigPath() string {
	var base string
	if runtime.GOOS == "windows" {
		base = os.Getenv("APPDATA")
		if base == "" {
			base = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
	} else {
		base = os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, "danksearch", "config.toml")
}

func (c *Config) findIndexPath(path string) *IndexPath {
	for i := range c.IndexPaths {
		if strings.HasPrefix(path, c.IndexPaths[i].Path) {
			return &c.IndexPaths[i]
		}
	}
	return nil
}

func (c *Config) ShouldIndexFile(path string) bool {
	idxPath := c.findIndexPath(path)
	if idxPath == nil {
		return false
	}

	if idxPath.ExcludeHidden && containsHiddenComponent(path, idxPath.Path) {
		return false
	}

	if containsExcludedComponent(path, idxPath.Path, idxPath.excludeDirsMap) {
		return false
	}

	return c.IndexAllFiles
}

func (c *Config) ShouldIndexDir(path string) bool {
	idxPath := c.findIndexPath(path)
	if idxPath == nil {
		return false
	}

	if idxPath.ExcludeHidden && containsHiddenComponent(path, idxPath.Path) {
		return false
	}

	return !containsExcludedComponent(path, idxPath.Path, idxPath.excludeDirsMap)
}

func containsHiddenComponent(path, rootDir string) bool {
	rel, err := filepath.Rel(rootDir, path)
	if err != nil {
		return false
	}

	if rel == "." {
		return false
	}

	parts := filepath.SplitList(filepath.ToSlash(rel))
	for _, part := range parts {
		if len(part) > 0 && part[0] == '.' {
			return true
		}
	}

	components := []string{}
	for p := rel; p != "."; p = filepath.Dir(p) {
		components = append([]string{filepath.Base(p)}, components...)
		if p == filepath.Dir(p) {
			break
		}
	}

	for _, comp := range components {
		if len(comp) > 0 && comp[0] == '.' {
			return true
		}
	}

	return false
}

func containsExcludedComponent(path, rootDir string, excludeMap map[string]bool) bool {
	rel, err := filepath.Rel(rootDir, path)
	if err != nil {
		return false
	}

	if rel == "." {
		return false
	}

	// Check each component in the path
	components := []string{}
	for p := rel; p != "."; p = filepath.Dir(p) {
		components = append([]string{filepath.Base(p)}, components...)
		if p == filepath.Dir(p) {
			break
		}
	}

	for _, comp := range components {
		if excludeMap[comp] {
			return true
		}
	}

	return false
}

func (c *Config) GetDepth(path string) int {
	idxPath := c.findIndexPath(path)
	if idxPath == nil {
		return 0
	}

	rel, err := filepath.Rel(idxPath.Path, path)
	if err != nil {
		return 0
	}

	if rel == "." {
		return 0
	}

	depth := 0
	for p := rel; p != "."; p = filepath.Dir(p) {
		depth++
		if p == filepath.Dir(p) {
			break
		}
	}

	return depth
}

func (c *Config) GetMaxDepth(path string) int {
	idxPath := c.findIndexPath(path)
	if idxPath == nil {
		return 0
	}
	return idxPath.MaxDepth
}

func (c *Config) IsTextFile(path string) bool {
	ext := filepath.Ext(path)
	return c.textExtsMap[ext]
}

type IndexStats struct {
	TotalFiles    int       `json:"total_files"`
	TotalBytes    int64     `json:"total_bytes"`
	LastIndexTime time.Time `json:"last_index_time"`
	IndexDuration string    `json:"index_duration"`
}
