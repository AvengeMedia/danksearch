package indexer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AvengeMedia/danksearch/internal/config"
	"github.com/stretchr/testify/suite"
)

type ExifAdvancedSuite struct {
	suite.Suite
	tmpDir string
	cfg    *config.Config
}

func TestExifAdvancedSuite(t *testing.T) {
	suite.Run(t, new(ExifAdvancedSuite))
}

func (s *ExifAdvancedSuite) SetupTest() {
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

func (s *ExifAdvancedSuite) newIndexerWithFile() *Indexer {
	idx, err := New(s.cfg)
	s.Require().NoError(err)
	s.T().Cleanup(func() { idx.Close() })

	testFile := filepath.Join(s.tmpDir, "test.txt")
	s.Require().NoError(os.WriteFile(testFile, []byte("test"), 0644))
	s.Require().NoError(idx.Index(testFile))

	return idx
}

func (s *ExifAdvancedSuite) TestSearchWithExifDateRange() {
	idx := s.newIndexerWithFile()
	opts := &SearchOptions{
		Query:          "*",
		Limit:          10,
		ExifDateAfter:  "2024-01-01T00:00:00Z",
		ExifDateBefore: "2024-12-31T23:59:59Z",
	}
	_, err := idx.SearchWithOptions(opts)
	s.NoError(err)
}

func (s *ExifAdvancedSuite) TestSearchWithISORange() {
	idx := s.newIndexerWithFile()
	opts := &SearchOptions{
		Query:      "*",
		Limit:      10,
		ExifMinISO: 100,
		ExifMaxISO: 3200,
	}
	_, err := idx.SearchWithOptions(opts)
	s.NoError(err)
}

func (s *ExifAdvancedSuite) TestSearchWithApertureRange() {
	idx := s.newIndexerWithFile()
	opts := &SearchOptions{
		Query:           "*",
		Limit:           10,
		ExifMinAperture: 1.8,
		ExifMaxAperture: 5.6,
	}
	_, err := idx.SearchWithOptions(opts)
	s.NoError(err)
}

func (s *ExifAdvancedSuite) TestSearchWithFocalLengthRange() {
	idx := s.newIndexerWithFile()
	opts := &SearchOptions{
		Query:           "*",
		Limit:           10,
		ExifMinFocalLen: 24,
		ExifMaxFocalLen: 200,
	}
	_, err := idx.SearchWithOptions(opts)
	s.NoError(err)
}

func (s *ExifAdvancedSuite) TestSearchWithGPSRange() {
	idx := s.newIndexerWithFile()
	opts := &SearchOptions{
		Query:      "*",
		Limit:      10,
		ExifLatMin: 40.0,
		ExifLatMax: 41.0,
		ExifLonMin: -74.0,
		ExifLonMax: -73.0,
	}
	_, err := idx.SearchWithOptions(opts)
	s.NoError(err)
}

func (s *ExifAdvancedSuite) TestSortByExifDate() {
	idx := s.newIndexerWithFile()
	opts := &SearchOptions{
		Query:    "*",
		Limit:    10,
		SortBy:   "exif_date",
		SortDesc: true,
	}
	_, err := idx.SearchWithOptions(opts)
	s.NoError(err)
}

func (s *ExifAdvancedSuite) TestSortByISO() {
	idx := s.newIndexerWithFile()
	opts := &SearchOptions{
		Query:    "*",
		Limit:    10,
		SortBy:   "iso",
		SortDesc: false,
	}
	_, err := idx.SearchWithOptions(opts)
	s.NoError(err)
}

func (s *ExifAdvancedSuite) TestSortByFocalLength() {
	idx := s.newIndexerWithFile()
	opts := &SearchOptions{
		Query:    "*",
		Limit:    10,
		SortBy:   "focal_length",
		SortDesc: true,
	}
	_, err := idx.SearchWithOptions(opts)
	s.NoError(err)
}
