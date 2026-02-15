package metastore

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

var bucketName = []byte("files")

type Store struct {
	db *bolt.DB
}

type FileMeta struct {
	ModTime time.Time
	Size    int64
}

func New(indexPath string) (*Store, error) {
	dbPath := filepath.Join(filepath.Dir(indexPath), "meta.db")
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		return err
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Put(path string, meta FileMeta) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		return b.Put([]byte(path), encodeMeta(meta))
	})
}

func (s *Store) Get(path string) (FileMeta, bool, error) {
	var meta FileMeta
	var found bool

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
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
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		return b.Delete([]byte(path))
	})
}

func (s *Store) ForEach(fn func(path string, meta FileMeta) error) error {
	return s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		return b.ForEach(func(k, v []byte) error {
			return fn(string(k), decodeMeta(v))
		})
	})
}

func (s *Store) ForEachPrefix(prefix string, fn func(path string, meta FileMeta) error) error {
	pfx := []byte(prefix)
	return s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucketName).Cursor()
		for k, v := c.Seek(pfx); k != nil && bytes.HasPrefix(k, pfx); k, v = c.Next() {
			if err := fn(string(k), decodeMeta(v)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) Clear() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(bucketName); err != nil {
			return err
		}
		_, err := tx.CreateBucket(bucketName)
		return err
	})
}

func (s *Store) Count() (int, error) {
	var count int
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		count = b.Stats().KeyN
		return nil
	})
	return count, err
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
