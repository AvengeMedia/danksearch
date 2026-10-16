package indexer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParentDirs(t *testing.T) {
	assert.Equal(t, []string{"/home/u/docs", "/home/u", "/home", "/"}, ParentDirs("/home/u/Docs/Report.pdf"))
	assert.Equal(t, []string{"/"}, ParentDirs("/top"))
	assert.Equal(t, "/", folderTerm("/"))
	assert.Equal(t, "/home/u", folderTerm("/home/U/"))
}
