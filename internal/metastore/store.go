package metastore

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/AvengeMedia/dankgo/log"
	bolt "go.etcd.io/bbolt"
)

var bucketName = []byte("files")
var schemaBucket = []byte("schema")

type Store struct {
	db *bolt.DB
}

type FileMeta struct {
	ModTime time.Time
	Size    int64
}

func New(indexPath string) (*Store, error) {
	dbPath := filepath.Join(filepath.Dir(indexPath), "meta.db")

	db, err := tryOpen(dbPath)
	if err == nil {
		return &Store{db: db}, nil
	}

	log.Errorf("metastore corrupted, salvaging readable entries: %v", err)

	salvagePath := dbPath + ".salvage"
	os.Remove(salvagePath)
	// chunked commits keep entries copied before the first unreadable page
	if salvageErr := compactInto(dbPath, salvagePath, 64<<10); salvageErr != nil {
		log.Warnf("metastore salvage stopped early: %v", salvageErr)
	}

	corruptPath := dbPath + ".corrupt"
	os.Remove(corruptPath)
	if renameErr := os.Rename(dbPath, corruptPath); renameErr != nil {
		os.Remove(salvagePath)
		return nil, err
	}

	os.Rename(salvagePath, dbPath)

	db, err = tryOpen(dbPath)
	if err == nil {
		return &Store{db: db}, nil
	}

	os.Remove(dbPath)
	db, err = tryOpen(dbPath)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// bbolt reports corrupted pages by panicking (internal/common/verify.go), not
// by returning errors, so every db touchpoint recovers and converts to error.
func recoverDBPanic(err *error) {
	r := recover()
	if r == nil {
		return
	}
	*err = fmt.Errorf("metastore db panic: %v", r)
}

// a pgid past the mmap end faults instead of panicking; only recoverable while armed
func armDBFaultPanics() func() {
	prev := debug.SetPanicOnFault(true)
	return func() { debug.SetPanicOnFault(prev) }
}

func tryOpen(path string) (db *bolt.DB, err error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		if db != nil {
			db.Close()
		}
		db, err = nil, fmt.Errorf("metastore db panic: %v", r)
	}()
	defer armDBFaultPanics()()

	db, err = bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketName); err != nil {
			return err
		}
		_, err := tx.CreateBucketIfNotExists(schemaBucket)
		return err
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	err = db.View(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{bucketName, schemaBucket} {
			b := tx.Bucket(name)
			if b == nil {
				continue
			}
			if err := b.ForEach(func(k, v []byte) error { return nil }); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func compactInto(srcPath, dstPath string, txMaxSize int64) (err error) {
	defer recoverDBPanic(&err)
	defer armDBFaultPanics()()

	srcDB, err := bolt.Open(srcPath, 0600, &bolt.Options{ReadOnly: true, Timeout: time.Second})
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcDB.Close()

	dstDB, err := bolt.Open(dstPath, 0600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return fmt.Errorf("open destination: %w", err)
	}
	defer dstDB.Close()

	if err := bolt.Compact(dstDB, srcDB, txMaxSize); err != nil {
		return fmt.Errorf("compact: %w", err)
	}
	return nil
}

func (s *Store) update(fn func(tx *bolt.Tx) error) (err error) {
	defer recoverDBPanic(&err)
	defer armDBFaultPanics()()
	return s.db.Update(fn)
}

func (s *Store) view(fn func(tx *bolt.Tx) error) (err error) {
	defer recoverDBPanic(&err)
	defer armDBFaultPanics()()
	return s.db.View(fn)
}

func (s *Store) Put(path string, meta FileMeta) error {
	return s.update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return fmt.Errorf("files bucket missing")
		}
		return b.Put([]byte(path), encodeMeta(meta))
	})
}

func (s *Store) Get(path string) (FileMeta, bool, error) {
	var meta FileMeta
	var found bool

	err := s.view(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return nil
		}
		v := b.Get([]byte(path))
		if v != nil {
			meta = decodeMeta(v)
			found = true
		}
		return nil
	})

	return meta, found, err
}

func (s *Store) Delete(path string) error {
	return s.update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return nil
		}
		return b.Delete([]byte(path))
	})
}

func (s *Store) ForEach(fn func(path string, meta FileMeta) error) error {
	return s.view(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			return fn(string(k), decodeMeta(v))
		})
	})
}

func (s *Store) ForEachPrefix(prefix string, fn func(path string, meta FileMeta) error) error {
	pfx := []byte(prefix)
	return s.view(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return nil
		}
		c := b.Cursor()
		for k, v := c.Seek(pfx); k != nil && bytes.HasPrefix(k, pfx); k, v = c.Next() {
			if err := fn(string(k), decodeMeta(v)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) Clear() error {
	return s.update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(bucketName); err != nil {
			return err
		}
		_, err := tx.CreateBucket(bucketName)
		return err
	})
}

func (s *Store) Count() (int, error) {
	var count int
	err := s.view(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return nil
		}
		count = b.Stats().KeyN
		return nil
	})
	return count, err
}

func (s *Store) GetMeta(key string) (string, error) {
	var val string
	err := s.view(func(tx *bolt.Tx) error {
		b := tx.Bucket(schemaBucket)
		if b == nil {
			return nil
		}
		v := b.Get([]byte(key))
		if v != nil {
			val = string(v)
		}
		return nil
	})
	return val, err
}

func (s *Store) PutMeta(key, value string) error {
	return s.update(func(tx *bolt.Tx) error {
		b := tx.Bucket(schemaBucket)
		if b == nil {
			return fmt.Errorf("schema bucket missing")
		}
		return b.Put([]byte(key), []byte(value))
	})
}

func (s *Store) Close() error {
	return s.db.Close()
}

func encodeMeta(m FileMeta) []byte {
	buf := make([]byte, 16)
	binary.LittleEndian.PutUint64(buf[0:8], uint64(m.ModTime.UnixNano()))
	binary.LittleEndian.PutUint64(buf[8:16], uint64(m.Size))
	return buf
}

func decodeMeta(b []byte) FileMeta {
	if len(b) < 16 {
		return FileMeta{}
	}
	return FileMeta{
		ModTime: time.Unix(0, int64(binary.LittleEndian.Uint64(b[0:8]))),
		Size:    int64(binary.LittleEndian.Uint64(b[8:16])),
	}
}
