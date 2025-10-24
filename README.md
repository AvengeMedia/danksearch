<div align="center">

<img src="assets/dank.svg" alt="DANK" width="400">

</div>

# dsearch

A configurable filesystem search service powered by [bleve](https://github.com/blevesearch/bleve) for GNU/Linux.

## Features

- **Indexing** - Indexes all files in specified paths
- **Configuration** - Configure exclusions, depth, etc.
- **Fuzzy search** - Typo-tolerant search queries
- **Text content extraction** - Optionally extracts and indexes content from text-based files
- **Concurrent indexing** - Multi-worker parallel indexing for fast performance
- **Real-time updates** - File watcher for incremental index updates

## Installation

Requires [Go](https://go.dev) 1.24+

```bash
make && make install && make install-service
```

## Usage

Best to run as a systemd user unit:

```bash
systemctl enable --now dsearch
```

Then search with various options:

```bash
# Basic search
dsearch search "golang"

# Show all results
dsearch search "*.md" --limit 0

# JSON output for scripting
dsearch search "config" --json
dsearch search "README" --limit 0 --json | jq '.hits[].id'

# Advanced options
dsearch search "function" --field filename --ext .go
dsearch search "typo" --fuzzy
dsearch search "*" --sort mtime --desc
# Scores content higher than filenames
dsearch search --field body mountain
```

For all options:
```bash
dsearch search --help
```

Or visit [http://localhost:43654/docs](http://localhost:43654/docs) for rest API documentation.

## Configuration

See [config.example.toml](./config.example.toml) for configuration options.

On startup, a configuration file will be created at `~/.config/dsearch/config.toml`