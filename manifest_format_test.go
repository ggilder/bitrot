package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestManifestFormatRoundTrip(t *testing.T) {
	entries := map[string]ChecksumRecord{
		"foo/bar.txt": {Checksum: "abc123", ModTime: time.Unix(0, 1700000000123456789).UTC()},
		"baz.txt":     {Checksum: "def456", ModTime: time.Unix(0, 1600000000000000000).UTC()},
	}

	var buf bytes.Buffer
	assert.Nil(t, WriteManifest(&buf, entries))

	roundTripped, err := ReadManifest(&buf)
	assert.Nil(t, err)
	assert.Equal(t, entries, roundTripped)
}

func TestManifestFormatEmpty(t *testing.T) {
	var buf bytes.Buffer
	assert.Nil(t, WriteManifest(&buf, map[string]ChecksumRecord{}))

	roundTripped, err := ReadManifest(&buf)
	assert.Nil(t, err)
	assert.Empty(t, roundTripped)
}

// This is the whole point of length-prefixing the path field: a path
// containing bytes that would otherwise collide with record/field
// delimiters (newlines, spaces) must still round-trip exactly.
func TestManifestFormatPathWithSpecialBytes(t *testing.T) {
	weirdPaths := []string{
		"has a space.txt",
		"has\na newline.txt",
		"has\ttabs\tin\tit.txt",
		"trailing-newline-in-name\n",
		"multiple\n\nnewlines\nin\na\nrow",
	}

	entries := map[string]ChecksumRecord{}
	for _, p := range weirdPaths {
		entries[p] = ChecksumRecord{Checksum: "somehash", ModTime: time.Unix(0, 123).UTC()}
	}

	var buf bytes.Buffer
	assert.Nil(t, WriteManifest(&buf, entries))

	roundTripped, err := ReadManifest(&buf)
	assert.Nil(t, err)
	assert.Equal(t, entries, roundTripped)
}

func TestReadManifestRejectsMalformedHeader(t *testing.T) {
	_, err := ReadManifest(bytes.NewBufferString("not a valid header\n"))
	assert.NotNil(t, err)
}

func TestReadManifestRejectsTruncatedPath(t *testing.T) {
	_, err := ReadManifest(bytes.NewBufferString("abc123 1700000000000000000 100\nshort\n"))
	assert.NotNil(t, err)
}
