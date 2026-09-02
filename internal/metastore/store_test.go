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
	berrors "go.etcd.io/bbolt/errors"
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

func TestNew_LockedDBIsNotTreatedAsCorrupt(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index")
	holder, err := New(indexPath)
	require.NoError(t, err)
	defer holder.Close()
	require.NoError(t, holder.Put("kept", FileMeta{Size: 1}))

	_, err = New(indexPath)
	require.ErrorIs(t, err, berrors.ErrTimeout)

	_, statErr := os.Stat(filepath.Join(dir, "meta.db.corrupt"))
	assert.True(t, os.IsNotExist(statErr))
	_, found, err := holder.Get("kept")
	require.NoError(t, err)
	assert.True(t, found)
}

func TestBatchOperations(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "index"))
	require.NoError(t, err)
	defer s.Close()

	require.NoError(t, s.PutBatch(map[string]FileMeta{
		"/a": {ModTime: time.Unix(1, 0), Size: 1},
		"/b": {ModTime: time.Unix(2, 0), Size: 2},
		"/c": {ModTime: time.Unix(3, 0), Size: 3},
	}))
	count, err := s.Count()
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	require.NoError(t, s.DeleteBatch([]string{"/a", "/c", "/missing"}))
	count, err = s.Count()
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	meta, found, err := s.Get("/b")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, int64(2), meta.Size)

	require.NoError(t, s.PutBatch(nil))
	require.NoError(t, s.DeleteBatch(nil))
}
