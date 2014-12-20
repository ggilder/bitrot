package checksum_generator

import (
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestDirectoryManifest(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	check(err)

	defer os.RemoveAll(tempDir)

	testFile := writeTestFile(tempDir, "foo", helloWorldString)
	subdir := filepath.Join(tempDir, "bar", "baz", "stuff")
	check(os.MkdirAll(subdir, 0755))
	testFile2 := writeTestFile(subdir, "foo", helloWorldString)

	expectedChecksums := map[string]string{
		testFile2: helloWorldChecksum,
		testFile:  helloWorldChecksum,
	}

	expectedCreationTime := time.Now()

	manifest := GenerateDirectoryManifest(tempDir)

	if manifest.path != tempDir {
		t.Fatalf("expected manifest path %s, got %s", tempDir, manifest.path)
	}

	if math.Abs(float64(manifest.createdAt.Unix()-expectedCreationTime.Unix())) > 5 {
		t.Fatalf("expected manifest createdAt within 5s of %v, got %v", expectedCreationTime, manifest.createdAt)
	}

	if len(manifest.entries) != len(expectedChecksums) {
		t.Fatalf(
			"unexpected number of checksums! expected %d, got %d (%v)",
			len(expectedChecksums),
			len(manifest.entries),
			manifest.entries,
		)
	}

	for path, fileChecksum := range manifest.entries {
		if fileChecksum.checksum != expectedChecksums[path] {
			t.Fatalf("checksum mismatch; expected %s, got %s", expectedChecksums[path], fileChecksum.checksum)
		}
	}
}
