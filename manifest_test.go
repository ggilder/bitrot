package main

// TODO refactor tests to use testify/assert library like `bitrot_test.go` and
// testify/suite to extract common before/after hooks
import (
	"encoding/json"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var helloWorldString = "hello! world\n"
var helloWorldChecksum = "87b3fe7479c73ae4246dbe8081550f52e2cf9e59"

func writeTestFile(t *testing.T, dir, name, content string) string {
	testFile := filepath.Join(dir, name)
	err := ioutil.WriteFile(testFile, []byte(content), 0644)
	assert.Nil(t, err)
	return testFile
}

func populateTestDirectory(t *testing.T, tempDir string) (map[string]string, time.Time) {
	writeTestFile(t, tempDir, "foo", helloWorldString)
	subdir := filepath.Join(tempDir, "bar", "baz", "stuff")
	assert.Nil(t, os.MkdirAll(subdir, 0755))
	writeTestFile(t, subdir, "foo", helloWorldString)

	expectedChecksums := map[string]string{
		"bar/baz/stuff/foo": helloWorldChecksum,
		"foo":               helloWorldChecksum,
	}

	expectedCreationTime := time.Now()

	return expectedChecksums, expectedCreationTime
}

func TestDirectoryManifest(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	assert.Nil(t, err)

	defer os.RemoveAll(tempDir)

	expectedChecksums, expectedCreationTime := populateTestDirectory(t, tempDir)

	config := Config{}
	manifest, err := NewManifest(tempDir, &config)
	assert.Nil(t, err)

	if manifest.Path != tempDir {
		t.Fatalf("expected manifest path %s, got %s", tempDir, manifest.Path)
	}

	if math.Abs(float64(manifest.CreatedAt.Unix()-expectedCreationTime.Unix())) > 5 {
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

func TestManifestExclusionOnFile(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	assert.Nil(t, err)

	defer os.RemoveAll(tempDir)

	populateTestDirectory(t, tempDir)

	config := Config{
		ExcludedFiles: []string{"foo"},
	}

	manifest, err := NewManifest(tempDir, &config)
	assert.Nil(t, err)

	if !reflect.DeepEqual(manifest.Entries, map[string]ChecksumRecord{}) {
		t.Fatalf("Entries mismatch; expected %v, got %v", map[string]ChecksumRecord{}, manifest.Entries)
	}
}

func TestManifestExclusionOnFolder(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	assert.Nil(t, err)

	defer os.RemoveAll(tempDir)

	populateTestDirectory(t, tempDir)

	config := Config{
		ExcludedFiles: []string{"baz"},
	}

	manifest, err := NewManifest(tempDir, &config)
	assert.Nil(t, err)

	entryPaths := []string{}
	for path := range manifest.Entries {
		entryPaths = append(entryPaths, path)
	}
	expectedEntryPaths := []string{"foo"}

	if !reflect.DeepEqual(entryPaths, expectedEntryPaths) {
		t.Fatalf("Entries mismatch; expected %v, got %v", expectedEntryPaths, entryPaths)
	}
}

func TestManifestJSON(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	assert.Nil(t, err)

	defer os.RemoveAll(tempDir)

	expectedChecksums, expectedCreationTime := populateTestDirectory(t, tempDir)

	config := Config{}
	manifest, err := NewManifest(tempDir, &config)
	assert.Nil(t, err)

	jsonBytes, err := json.Marshal(manifest)

	var recreatedManifest Manifest
	err = json.Unmarshal(jsonBytes, &recreatedManifest)
	assert.Nil(t, err)

	if recreatedManifest.Path != tempDir {
		t.Fatalf("expected JSON path %s, got %s", tempDir, recreatedManifest.Path)
	}

	if math.Abs(float64(recreatedManifest.CreatedAt.Unix()-expectedCreationTime.Unix())) > 5 {
		t.Fatalf("expected manifest created_at within 5s of %v, got %v", expectedCreationTime, recreatedManifest.CreatedAt)
	}

	entries := recreatedManifest.Entries
	if len(entries) != len(expectedChecksums) {
		t.Fatalf(
			"unexpected number of checksums! expected %d, got %d (%v)",
			len(expectedChecksums),
			len(entries),
			entries,
		)
	}

	for path, fileChecksum := range entries {
		if fileChecksum.Checksum != expectedChecksums[path] {
			t.Fatalf("checksum mismatch; expected %s, got %s", expectedChecksums[path], fileChecksum.Checksum)
		}
	}
}
