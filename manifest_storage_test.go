package main

import (
	// "fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

/*
TODO:
- Test manifest content
- Test "latest manifest" functionality
- Test list more thoroughly
- Edge cases
	- storage folder already exists but somehow has the wrong path in metadata
*/

func TestManifestStorage(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	assert.Nil(t, err)
	defer os.RemoveAll(tempDir)

	s := NewManifestStorage(tempDir)
	entries, err := s.List()
	assert.Nil(t, err)
	assert.Len(t, entries, 0)

	createdAt, err := time.Parse(time.RFC3339, "2019-01-30T22:08:41+00:00")
	testPath := "/foo/bar/baz"
	manifest := &Manifest{
		Path:      testPath,
		CreatedAt: createdAt,
	}
	assert.Nil(t, s.AddManifest(manifest))

	entries, err = s.List()
	assert.Nil(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, testPath, entries[0].Path)
}
