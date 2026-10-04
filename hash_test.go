package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func expectedSHA256(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func TestHashFileMatchesStdlibForVariousSizes(t *testing.T) {
	// Deliberately spans sizes smaller than, equal to, and spread across
	// multiple multiples of a small test buffer, to prove the read loop
	// handles partial reads and multi-chunk files correctly - not just the
	// single-read happy path.
	sizes := []int{0, 1, 15, 16, 17, 100}

	tempDir, err := ioutil.TempDir("", "hashfile")
	assert.Nil(t, err)
	defer os.RemoveAll(tempDir)

	buf := make([]byte, 16) // small on purpose, to force multiple reads

	for _, size := range sizes {
		content := make([]byte, size)
		for i := range content {
			content[i] = byte(i % 251)
		}

		path := tempDir + "/" + "file"
		assert.Nil(t, ioutil.WriteFile(path, content, 0644))

		checksum, err := hashFile(path, buf)
		assert.Nil(t, err)
		assert.Equal(t, expectedSHA256(content), checksum, "size=%d", size)
	}
}

func TestHashFileErrorsOnMissingFile(t *testing.T) {
	_, err := hashFile("/nonexistent/path/does/not/exist", make([]byte, 16))
	assert.NotNil(t, err)
}
