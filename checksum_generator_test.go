package checksum_generator

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(dir, name, content string) string {
	testFile := filepath.Join(dir, name)
	err := ioutil.WriteFile(testFile, []byte(content), 0400)
	check(err)
	return testFile
}

func TestFileChecksum(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	check(err)

	defer os.RemoveAll(tempDir)

	testFile := writeTestFile(tempDir, "foo", "hello! world\n")
	fileChecksum := FileChecksum(testFile)
	var fi os.FileInfo
	fi, err = os.Stat(testFile)

	correctModTime := fi.ModTime()
	correctSum := "87b3fe7479c73ae4246dbe8081550f52e2cf9e59"
	if fileChecksum.path != testFile {
		t.Fatalf("expected path %s; got %s", testFile, fileChecksum.path)
	}
	if fileChecksum.checksum != correctSum {
		t.Fatalf("expected checksum %s; got %s", correctSum, fileChecksum.checksum)
	}
	if fileChecksum.modTime != correctModTime {
		t.Fatalf("expected modTime %s; got %s", correctModTime, fileChecksum.modTime)
	}
}
