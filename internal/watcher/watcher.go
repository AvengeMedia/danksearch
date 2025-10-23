package watcher

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/AvengeMedia/danksearch/internal/config"
	"github.com/AvengeMedia/danksearch/internal/errdefs"
	"github.com/AvengeMedia/danksearch/internal/log"
	"github.com/fsnotify/fsnotify"
)

type Indexer interface {
	Index(path string) error
	Delete(path string) error
}

type Watcher struct {
	watcher *fsnotify.Watcher
	indexer Indexer
	config  *config.Config
	running bool
	mu      sync.Mutex
	done    chan struct{}
}

func New(indexer Indexer, cfg *config.Config) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, errdefs.NewCustomError(errdefs.ErrTypeWatcherFailed, "failed to create watcher", err)
	}

	return &Watcher{
		watcher: w,
		indexer: indexer,
		config:  cfg,
		done:    make(chan struct{}),
	}, nil
}

func (w *Watcher) Start() error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}

	// Create a new watcher if the previous one was closed
	if w.watcher == nil {
		newWatcher, err := fsnotify.NewWatcher()
		if err != nil {
			w.mu.Unlock()
			return errdefs.NewCustomError(errdefs.ErrTypeWatcherFailed, "failed to create watcher", err)
		}
		w.watcher = newWatcher
		w.done = make(chan struct{})
	}

	w.running = true
	w.mu.Unlock()

	for _, idxPath := range w.config.IndexPaths {
		if err := w.addWatches(idxPath.Path); err != nil {
			return err
		}
	}

	go w.eventLoop()
	log.Infof("watcher started")
	return nil
}

func (w *Watcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return nil
	}

	w.running = false
	close(w.done)
	err := w.watcher.Close()
	w.watcher = nil // Allow recreation on next Start()
	log.Infof("watcher stopped")
	return err
}

func (w *Watcher) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

func (w *Watcher) addWatches(root string) error {
	watchCount := 0
	errorCount := 0

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				log.Debugf("permission denied: %s", path)
				return nil
			}
			return err
		}

		if !info.IsDir() {
			return nil
		}

		if !w.config.ShouldIndexDir(path) {
			return filepath.SkipDir
		}

		depth := w.config.GetDepth(path)
		maxDepth := w.config.GetMaxDepth(path)
		if maxDepth > 0 && depth >= maxDepth {
			return filepath.SkipDir
		}

		if err := w.watcher.Add(path); err != nil {
			errorCount++
			if errorCount == 1 {
				log.Warnf("failed to add watch for %s: %v", path, err)
			}
			return nil
		}

		watchCount++
		return nil
	})

	if errorCount > 0 {
		log.Warnf("failed to add %d watches (added %d successfully)", errorCount, watchCount)
		log.Infof("if you hit inotify limits, increase with: sudo sysctl fs.inotify.max_user_watches=524288")
	} else {
		log.Infof("added %d directory watches", watchCount)
	}

	return err
}

func (w *Watcher) eventLoop() {
	for {
		select {
		case <-w.done:
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Errorf("watcher error: %v", err)
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	path := event.Name

	if event.Op&fsnotify.Create == fsnotify.Create {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			if err := w.watcher.Add(path); err != nil {
				log.Debugf("failed to watch new dir %s: %v", path, err)
			}
			return
		}

		if w.config.ShouldIndexFile(path) {
			if err := w.indexer.Index(path); err != nil {
				log.Debugf("failed to index %s: %v", path, err)
			}
		}
	}

	if event.Op&fsnotify.Write == fsnotify.Write {
		if w.config.ShouldIndexFile(path) {
			if err := w.indexer.Index(path); err != nil {
				log.Debugf("failed to reindex %s: %v", path, err)
			}
		}
	}

	if event.Op&fsnotify.Remove == fsnotify.Remove {
		if err := w.indexer.Delete(path); err != nil {
			log.Debugf("failed to delete %s: %v", path, err)
		}
	}

	if event.Op&fsnotify.Rename == fsnotify.Rename {
		if err := w.indexer.Delete(path); err != nil {
			log.Debugf("failed to delete renamed file %s: %v", path, err)
		}
	}
}
