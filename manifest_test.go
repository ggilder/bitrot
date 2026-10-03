package main

// TODO refactor tests to use testify/assert library like `bitrot_test.go` and
// testify/suite to extract common before/after hooks
import (
	"bytes"
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
var helloWorldChecksum = "dff770fab8b569686bad419a25a97033f90e89943dea564a91d6a4e7327fbfa9"

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
	manifest, errored, err := NewManifest(tempDir, &config, 2)
	assert.Nil(t, err)
	assert.Empty(t, errored)

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

func TestManifestExclusionOnName(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	assert.Nil(t, err)

	defer os.RemoveAll(tempDir)

	populateTestDirectory(t, tempDir)

	config := Config{
		ExcludedNames: []string{"foo"},
	}

	manifest, errored, err := NewManifest(tempDir, &config, 2)
	assert.Nil(t, err)
	assert.Empty(t, errored)

	if !reflect.DeepEqual(manifest.Entries, map[string]ChecksumRecord{}) {
		t.Fatalf("Entries mismatch; expected %v, got %v", map[string]ChecksumRecord{}, manifest.Entries)
	}
}

func TestManifestExclusionCoversAllBuiltInDefaultNames(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	assert.Nil(t, err)

	defer os.RemoveAll(tempDir)

	writeTestFile(t, tempDir, "kept.txt", helloWorldString)
	for _, name := range defaultExcludedNames {
		writeTestFile(t, tempDir, name, "should be excluded")
	}

	config := DefaultConfig()
	manifest, errored, err := NewManifest(tempDir, config, 2)
	assert.Nil(t, err)
	assert.Empty(t, errored)

	entryPaths := []string{}
	for path := range manifest.Entries {
		entryPaths = append(entryPaths, path)
	}
	assert.Equal(t, []string{"kept.txt"}, entryPaths)
}

func TestManifestExclusionOnFolder(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	assert.Nil(t, err)

	defer os.RemoveAll(tempDir)

	populateTestDirectory(t, tempDir)

	config := Config{
		ExcludedNames: []string{"baz"},
	}

	manifest, errored, err := NewManifest(tempDir, &config, 2)
	assert.Nil(t, err)
	assert.Empty(t, errored)

	entryPaths := []string{}
	for path := range manifest.Entries {
		entryPaths = append(entryPaths, path)
	}
	expectedEntryPaths := []string{"foo"}

	if !reflect.DeepEqual(entryPaths, expectedEntryPaths) {
		t.Fatalf("Entries mismatch; expected %v, got %v", expectedEntryPaths, entryPaths)
	}
}

func TestManifestExclusionOnPrefix(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	assert.Nil(t, err)

	defer os.RemoveAll(tempDir)

	populateTestDirectory(t, tempDir)

	config := Config{
		ExcludedPrefixes: []string{"bar/baz"},
	}

	manifest, errored, err := NewManifest(tempDir, &config, 2)
	assert.Nil(t, err)
	assert.Empty(t, errored)

	entryPaths := []string{}
	for path := range manifest.Entries {
		entryPaths = append(entryPaths, path)
	}
	expectedEntryPaths := []string{"foo"}

	if !reflect.DeepEqual(entryPaths, expectedEntryPaths) {
		t.Fatalf("Entries mismatch; expected %v, got %v", expectedEntryPaths, entryPaths)
	}
}

func TestManifestExclusionOnPrefixDoesNotMatchSimilarSiblingNames(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	assert.Nil(t, err)

	defer os.RemoveAll(tempDir)

	writeTestFile(t, tempDir, "foo", helloWorldString)
	assert.Nil(t, os.MkdirAll(filepath.Join(tempDir, "bar"), 0755))
	assert.Nil(t, os.MkdirAll(filepath.Join(tempDir, "barbecue"), 0755))
	writeTestFile(t, filepath.Join(tempDir, "bar"), "excluded", helloWorldString)
	writeTestFile(t, filepath.Join(tempDir, "barbecue"), "included", helloWorldString)

	config := Config{
		ExcludedPrefixes: []string{"bar"},
	}

	manifest, errored, err := NewManifest(tempDir, &config, 2)
	assert.Nil(t, err)
	assert.Empty(t, errored)

	entryPaths := []string{}
	for path := range manifest.Entries {
		entryPaths = append(entryPaths, path)
	}

	assert.ElementsMatch(t, []string{"foo", "barbecue/included"}, entryPaths)
}

func TestManifestRoundTrip(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	assert.Nil(t, err)

	defer os.RemoveAll(tempDir)

	expectedChecksums, _ := populateTestDirectory(t, tempDir)

	config := Config{}
	manifest, _, err := NewManifest(tempDir, &config, 2)
	assert.Nil(t, err)

	var buf bytes.Buffer
	assert.Nil(t, WriteManifest(&buf, manifest.Entries))

	roundTripped, err := ReadManifest(&buf)
	assert.Nil(t, err)

	assert.Len(t, roundTripped, len(expectedChecksums))
	for path, fileChecksum := range roundTripped {
		if fileChecksum.Checksum != expectedChecksums[path] {
			t.Fatalf("checksum mismatch; expected %s, got %s", expectedChecksums[path], fileChecksum.Checksum)
		}
	}
}
