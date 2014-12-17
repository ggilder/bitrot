package checksum_generator

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

var helloWorldString = "hello! world\n"
var helloWorldChecksum = "87b3fe7479c73ae4246dbe8081550f52e2cf9e59"

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

	testFile := writeTestFile(tempDir, "foo", helloWorldString)
	fileChecksum := FileChecksum(testFile)
	var fi os.FileInfo
	fi, err = os.Stat(testFile)

	correctModTime := fi.ModTime()
	if fileChecksum.path != testFile {
		t.Fatalf("expected path %s; got %s", testFile, fileChecksum.path)
	}
	if fileChecksum.checksum != helloWorldChecksum {
		t.Fatalf("expected checksum %s; got %s", helloWorldChecksum, fileChecksum.checksum)
	}
	if fileChecksum.modTime != correctModTime {
		t.Fatalf("expected modTime %s; got %s", correctModTime, fileChecksum.modTime)
	}
}

func TestDirectoryChecksum(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	check(err)

	defer os.RemoveAll(tempDir)

	testFile := writeTestFile(tempDir, "foo", helloWorldString)
	subdir := filepath.Join(tempDir, "bar", "baz", "stuff")
	check(os.MkdirAll(subdir, 0755))
	testFile2 := writeTestFile(subdir, "foo", helloWorldString)

	expectedChecksums := []struct {
		path     string
		checksum string
	}{
		{testFile2, helloWorldChecksum},
		{testFile, helloWorldChecksum},
	}

	checksums := DirectoryChecksums(tempDir)

	if len(checksums) != len(expectedChecksums) {
		t.Fatalf(
			"unexpected number of checksums! expected %d, got %d (%v)",
			len(expectedChecksums),
			len(checksums),
			checksums,
		)
	}

	for i, fileChecksum := range checksums {
		if fileChecksum.path != expectedChecksums[i].path {
			t.Fatalf("path mismatch; expected %s, got %s", expectedChecksums[i].path, fileChecksum.path)
		}
		if fileChecksum.checksum != expectedChecksums[i].checksum {
			t.Fatalf("checksum mismatch; expected %s, got %s", expectedChecksums[i].checksum, fileChecksum.checksum)
		}
	}
}
