package checksum_generator

import (
	"encoding/json"
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

func populateTestDirectory(tempDir string) (map[string]string, time.Time) {
	writeTestFile(tempDir, "foo", helloWorldString)
	subdir := filepath.Join(tempDir, "bar", "baz", "stuff")
	check(os.MkdirAll(subdir, 0755))
	writeTestFile(subdir, "foo", helloWorldString)

	expectedChecksums := map[string]string{
		"bar/baz/stuff/foo": helloWorldChecksum,
		"foo":               helloWorldChecksum,
	}

	expectedCreationTime := time.Now()

	return expectedChecksums, expectedCreationTime
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
	if fileChecksum.Checksum != helloWorldChecksum {
		t.Fatalf("expected checksum %s; got %s", helloWorldChecksum, fileChecksum.Checksum)
	}
	if fileChecksum.ModTime != correctModTime {
		t.Fatalf("expected modTime %s; got %s", correctModTime, fileChecksum.ModTime)
	}
}

func TestDirectoryManifest(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	check(err)

	defer os.RemoveAll(tempDir)

	expectedChecksums, expectedCreationTime := populateTestDirectory(tempDir)

	manifest := GenerateDirectoryManifest(tempDir)

	if manifest.Path != tempDir {
		t.Fatalf("expected manifest path %s, got %s", tempDir, manifest.Path)
	}

	if math.Abs(float64(manifest.CreatedAt.Unix()-expectedCreationTime.Unix())) > 0 {
		t.Fatalf("expected manifest createdAt within 5s of %v, got %v", expectedCreationTime, manifest.CreatedAt)
	}

	if len(manifest.Entries) != len(expectedChecksums) {
		t.Fatalf(
			"unexpected number of checksums! expected %d, got %d (%v)",
			len(expectedChecksums),
			len(manifest.Entries),
			manifest.Entries,
		)
	}

	for path, fileChecksum := range manifest.Entries {
		if fileChecksum.Checksum != expectedChecksums[path] {
			t.Fatalf("checksum mismatch; expected %s, got %s", expectedChecksums[path], fileChecksum.Checksum)
		}
	}
}

func TestManifestJSON(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	check(err)

	defer os.RemoveAll(tempDir)

	expectedChecksums, expectedCreationTime := populateTestDirectory(tempDir)

	manifest := GenerateDirectoryManifest(tempDir)

	var jsonBytes []byte
	jsonBytes, err = json.Marshal(manifest)

	var jsonData map[string]interface{}
	err = json.Unmarshal(jsonBytes, &jsonData)
	check(err)

	if jsonData["path"] != tempDir {
		t.Fatalf("expected JSON path %s, got %s", tempDir, jsonData["path"])
	}

	timeString, _ := jsonData["created_at"].(string)
	var parsedTime time.Time
	parsedTime, err = time.Parse(time.RFC3339, timeString)
	check(err)
	if math.Abs(float64(parsedTime.Unix()-expectedCreationTime.Unix())) > 0 {
		t.Fatalf("expected manifest created_at within 0s of %v, got %v", expectedCreationTime, parsedTime)
	}

	entries, _ := jsonData["entries"].(map[string]interface{})
	if len(entries) != len(expectedChecksums) {
		t.Fatalf(
			"unexpected number of checksums! expected %d, got %d (%v)",
			len(expectedChecksums),
			len(entries),
			entries,
		)
	}

	for path, fileChecksumJSON := range entries {
		fileChecksum, _ := fileChecksumJSON.(map[string]interface{})
		checksum, _ := fileChecksum["checksum"].(string)

		if checksum != expectedChecksums[path] {
			t.Fatalf("checksum mismatch; expected %s, got %s", expectedChecksums[path], checksum)
		}
	}
}
