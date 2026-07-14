package indexer

import (
	"time"

	"github.com/AvengeMedia/dankgo/log"
	"github.com/AvengeMedia/danksearch/internal/metastore"
)

const (
	defaultBatchSize     = 500
	defaultBatchInterval = 2 * time.Second
)

type batchJob struct {
	path  string
	doc   *Document
	size  int64
	mtime time.Time
}

type batcher struct {
	idx      *Indexer
	in       chan batchJob
	done     chan struct{}
	size     int
	interval time.Duration
}

func newBatcher(idx *Indexer, size int, interval time.Duration) *batcher {
	b := &batcher{
		idx:      idx,
		in:       make(chan batchJob, size*2),
		done:     make(chan struct{}),
		size:     size,
		interval: interval,
	}
	go b.run()
	return b
}

func (b *batcher) submit(job batchJob) {
	b.in <- job
}

func (b *batcher) close() {
	close(b.in)
	<-b.done
}

func (b *batcher) run() {
	defer close(b.done)

	b.idx.mu.RLock()
	idx := b.idx.index
	b.idx.mu.RUnlock()

	batch := idx.NewBatch()
	pending := make([]batchJob, 0, b.size)

	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	flush := func() {
		if len(pending) == 0 {
			return
		}
		switch err := idx.Batch(batch); {
		case err != nil:
			log.Warnf("batch submit failed for %d docs: %v", len(pending), err)
		default:
			for _, job := range pending {
				if err := b.idx.meta.Put(job.path, metastore.FileMeta{ModTime: job.mtime, Size: job.size}); err != nil {
					log.Debugf("failed to update metastore for %s: %v", job.path, err)
				}
			}
			b.idx.indexComplete.Store(true)
		}
		batch = idx.NewBatch()
		pending = pending[:0]
	}

	for {
		select {
		case job, ok := <-b.in:
			if !ok {
				flush()
				return
			}
			if err := batch.Index(job.path, job.doc); err != nil {
				log.Warnf("failed to stage %s in batch: %v", job.path, err)
				continue
			}
			pending = append(pending, job)
			if len(pending) >= b.size {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
