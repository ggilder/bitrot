package main

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBufferSizeConfiguration(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	assert.Nil(t, err)

	defer os.RemoveAll(tempDir)

	populateTestDirectory(t, tempDir)
	bufferSize := 1024
	path := writeTestFile(t, tempDir, "foo", helloWorldString)
	reader, err := newSha1Reader(path, bufferSize)
	assert.Nil(t, err)
	assert.Equal(t, 1024, reader.bufferSize)
}
