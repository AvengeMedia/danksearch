package metastore

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

// zero padding in a fresh db, never a valid page
const corruptRootPgid = 7

// the inline bucket value starts with the root pgid right after the key bytes
func corruptBucketRoot(t *testing.T, path string, pastEOF bool) {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	idx := bytes.Index(data, bucketName)
	require.NotEqual(t, -1, idx)
	require.Equal(t, idx, bytes.LastIndex(data, bucketName))

	f, err := os.OpenFile(path, os.O_RDWR, 0o644)
	require.NoError(t, err)
	defer f.Close()

	_, err = f.WriteAt([]byte{corruptRootPgid}, int64(idx+len(bucketName)))
	require.NoError(t, err)

	if pastEOF {
		require.NoError(t, os.Truncate(path, int64(corruptRootPgid)*int64(os.Getpagesize())))
	}

	_, err = tryOpen(path)
	require.Error(t, err)
}

func newCorruptStorePath(t *testing.T, pastEOF bool) string {
	t.Helper()

	dir := t.TempDir()
	s, err := New(filepath.Join(dir, "index"))
	require.NoError(t, err)
	require.NoError(t, s.Close())

	path := filepath.Join(dir, "meta.db")
	corruptBucketRoot(t, path, pastEOF)
	return path
}

var corruptDBCases = []struct {
	name    string
	pastEOF bool
}{
	{"root page unwritten", false},
	{"root page past end of file", true},
}

func TestNew_HealsCorruptedDB(t *testing.T) {
	for _, tt := range corruptDBCases {
		t.Run(tt.name, func(t *testing.T) {
			path := newCorruptStorePath(t, tt.pastEOF)

			s, err := New(filepath.Join(filepath.Dir(path), "index"))
			require.NoError(t, err)
			defer s.Close()

			require.NoError(t, s.Put("a", FileMeta{ModTime: time.Unix(2, 0), Size: 2}))
			meta, found, err := s.Get("a")
			require.NoError(t, err)
			require.True(t, found)
			assert.Equal(t, int64(2), meta.Size)
		})
	}
}

func TestUpdate_CorruptDBReturnsErrorNotPanic(t *testing.T) {
	for _, tt := range corruptDBCases {
		t.Run(tt.name, func(t *testing.T) {
			path := newCorruptStorePath(t, tt.pastEOF)

			db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
			require.NoError(t, err)
			defer db.Close()

			s := &Store{db: db}
			assert.Error(t, s.Put("a", FileMeta{ModTime: time.Unix(2, 0), Size: 2}))
			_, _, err = s.Get("a")
			assert.Error(t, err)
		})
	}
}
