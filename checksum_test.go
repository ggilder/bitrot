package main

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
)

func TestBufferSizeConfiguration(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	check(err)

	defer os.RemoveAll(tempDir)

	populateTestDirectory(tempDir)
	bufferSize := 1024
	path := writeTestFile(tempDir, "foo", helloWorldString)
	reader, err := newSha1Reader(path, bufferSize)
	check(err)
	assert.Equal(t, 1024, reader.bufferSize)
}
