package indexer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AvengeMedia/danksearch/internal/config"
	"github.com/stretchr/testify/suite"
)

type IndexerSuite struct {
	suite.Suite
	tmpDir string
	cfg    *config.Config
}

func TestIndexerSuite(t *testing.T) {
	suite.Run(t, new(IndexerSuite))
}

func (s *IndexerSuite) SetupTest() {
	s.tmpDir = s.T().TempDir()
	s.cfg = config.Default()
	s.cfg.IndexPath = filepath.Join(s.tmpDir, "index")
	s.cfg.IndexPaths = []config.IndexPath{
		{
			Path:          s.tmpDir,
			MaxDepth:      10,
			ExcludeHidden: false,
			ExcludeDirs:   []string{},
		},
	}
	s.cfg.BuildMaps()
}

func (s *IndexerSuite) newIndexer() *Indexer {
	idx, err := New(s.cfg)
	s.Require().NoError(err)
	s.T().Cleanup(func() { idx.Close() })
	return idx
}

func (s *IndexerSuite) writeFile(name, content string) string {
	path := filepath.Join(s.tmpDir, name)
	s.Require().NoError(os.WriteFile(path, []byte(content), 0644))
	return path
}

func (s *IndexerSuite) TestNew() {
	idx := s.newIndexer()
	s.NotNil(idx.index)
	s.Equal(s.cfg, idx.config)
}

func (s *IndexerSuite) TestIndex() {
	idx := s.newIndexer()
	testFile := s.writeFile("test.txt", "hello world")

	s.NoError(idx.Index(testFile))

	result, err := idx.Search("hello", 10)
	s.Require().NoError(err)
	s.NotZero(result.Total)
}

func (s *IndexerSuite) TestDelete() {
	idx := s.newIndexer()
	testFile := s.writeFile("test.txt", "hello world")

	s.Require().NoError(idx.Index(testFile))
	s.NoError(idx.Delete(testFile))

	result, err := idx.Search("hello", 10)
	s.Require().NoError(err)
	s.Zero(result.Total)
}

func (s *IndexerSuite) TestSearch() {
	idx := s.newIndexer()
	s.writeFile("test1.txt", "golang programming")
	s.writeFile("test2.txt", "python scripting")

	idx.Index(filepath.Join(s.tmpDir, "test1.txt"))
	idx.Index(filepath.Join(s.tmpDir, "test2.txt"))

	result, err := idx.Search("golang", 10)
	s.Require().NoError(err)
	s.NotZero(result.Total)

	result, err = idx.Search("python", 10)
	s.Require().NoError(err)
	s.NotZero(result.Total)
}

func (s *IndexerSuite) TestStats() {
	idx := s.newIndexer()
	s.NotNil(idx.Stats())
}

func (s *IndexerSuite) TestRemoveIndexDir() {
	idx, err := New(s.cfg)
	s.Require().NoError(err)
	idx.Close()

	s.NoError(removeIndexDir(s.cfg.IndexPath))
	_, statErr := os.Stat(s.cfg.IndexPath)
	s.True(os.IsNotExist(statErr))
}

func (s *IndexerSuite) TestRemoveIndexDirRefusesNonIndex() {
	plainDir := filepath.Join(s.tmpDir, "not-an-index")
	s.Require().NoError(os.MkdirAll(plainDir, 0755))
	s.Require().NoError(os.WriteFile(filepath.Join(plainDir, "important.txt"), []byte("data"), 0644))

	err := removeIndexDir(plainDir)
	s.Error(err)
	s.Contains(err.Error(), "not a bleve index directory")

	_, statErr := os.Stat(filepath.Join(plainDir, "important.txt"))
	s.NoError(statErr)
}

func (s *IndexerSuite) TestRemoveIndexDirNonExistent() {
	s.NoError(removeIndexDir(filepath.Join(s.tmpDir, "does-not-exist")))
}

func (s *IndexerSuite) TestRemoveIndexDirEmpty() {
	s.Error(removeIndexDir(""))
}

func (s *IndexerSuite) TestReadDocument() {
	idx := s.newIndexer()
	content := "package main\n\nfunc main() {}\n"
	testFile := s.writeFile("test.go", content)

	info, err := os.Stat(testFile)
	s.Require().NoError(err)

	doc, err := idx.readDocument(testFile, info)
	s.Require().NoError(err)
	s.Equal(testFile, doc.Path)
	s.Equal("test.go", doc.Filename)
	s.Equal(content, doc.Body)
	s.NotEmpty(doc.Hash)
}
