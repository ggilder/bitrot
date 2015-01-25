package main

// TODO refactor tests to use assert library like `bitrot_test.go`
import (
	"encoding/json"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

var helloWorldString = "hello! world\n"
var helloWorldChecksum = "87b3fe7479c73ae4246dbe8081550f52e2cf9e59"

func writeTestFile(dir, name, content string) string {
	testFile := filepath.Join(dir, name)
	err := ioutil.WriteFile(testFile, []byte(content), 0644)
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
	fileChecksum := ChecksumRecordForFile(testFile)
	var fi os.FileInfo
	fi, err = os.Stat(testFile)

	correctModTime := fi.ModTime().UTC()
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

	config := Config{}
	manifest := ManifestForPath(tempDir, &config)

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

func TestManifestExclusionOnFile(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	check(err)

	defer os.RemoveAll(tempDir)

	populateTestDirectory(tempDir)

	config := Config{
		ExcludedFiles: []string{"foo"},
	}

	manifest := ManifestForPath(tempDir, &config)

	if !reflect.DeepEqual(manifest.Entries, map[string]ChecksumRecord{}) {
		t.Fatalf("Entries mismatch; expected %v, got %v", map[string]ChecksumRecord{}, manifest.Entries)
	}
}

func TestManifestExclusionOnFolder(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checksum")
	check(err)

	defer os.RemoveAll(tempDir)

	populateTestDirectory(tempDir)

	config := Config{
		ExcludedFiles: []string{"baz"},
	}

	manifest := ManifestForPath(tempDir, &config)

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
	check(err)

	defer os.RemoveAll(tempDir)

	expectedChecksums, expectedCreationTime := populateTestDirectory(tempDir)

	config := Config{}
	manifest := ManifestForPath(tempDir, &config)

	jsonBytes, err := json.Marshal(manifest)

	var recreatedManifest Manifest
	err = json.Unmarshal(jsonBytes, &recreatedManifest)
	check(err)

	if recreatedManifest.Path != tempDir {
		t.Fatalf("expected JSON path %s, got %s", tempDir, recreatedManifest.Path)
	}

	if math.Abs(float64(recreatedManifest.CreatedAt.Unix()-expectedCreationTime.Unix())) > 0 {
		t.Fatalf("expected manifest created_at within 0s of %v, got %v", expectedCreationTime, recreatedManifest.CreatedAt)
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

func TestManifestComparison(t *testing.T) {
	newCreatedAt := time.Now()
	newModTime := time.Now()
	oldCreatedAt := newCreatedAt.Add(-24 * time.Hour)
	oldModTime := newModTime.Add(-24 * time.Hour)
	oldManifest := Manifest{
		Path:      "/old/stuff",
		CreatedAt: oldCreatedAt,
		Entries: map[string]ChecksumRecord{
			"silently_corrupted": ChecksumRecord{Checksum: "asdf", ModTime: oldModTime},
			"not_changed":        ChecksumRecord{Checksum: "zxcv", ModTime: oldModTime},
			"modified":           ChecksumRecord{Checksum: "qwer", ModTime: oldModTime},
			"touched":            ChecksumRecord{Checksum: "olkm", ModTime: oldModTime},
			"deleted":            ChecksumRecord{Checksum: "jklh", ModTime: oldModTime},
		},
	}

	newManifest := Manifest{
		Path:      "/new/thing",
		CreatedAt: newCreatedAt,
		Entries: map[string]ChecksumRecord{
			"silently_corrupted": ChecksumRecord{Checksum: "zzzz", ModTime: oldModTime},
			"not_changed":        ChecksumRecord{Checksum: "zxcv", ModTime: oldModTime},
			"modified":           ChecksumRecord{Checksum: "tyui", ModTime: newModTime},
			"touched":            ChecksumRecord{Checksum: "olkm", ModTime: newModTime},
			"added":              ChecksumRecord{Checksum: "bnmv", ModTime: newModTime},
		},
	}

	comparison := CompareManifests(oldManifest, newManifest)

	expectedDeletedPaths := []string{"deleted"}
	if !reflect.DeepEqual(comparison.DeletedPaths, expectedDeletedPaths) {
		t.Fatalf("expected DeletedPaths %v; got %v", expectedDeletedPaths, comparison.DeletedPaths)
	}

	expectedModifiedPaths := []string{"modified"}
	if !reflect.DeepEqual(comparison.ModifiedPaths, expectedModifiedPaths) {
		t.Fatalf("expected ModifiedPaths %v; got %v", expectedModifiedPaths, comparison.ModifiedPaths)
	}

	expectedFlaggedPaths := []string{"silently_corrupted"}
	if !reflect.DeepEqual(comparison.FlaggedPaths, expectedFlaggedPaths) {
		t.Fatalf("expected FlaggedPaths %v; got %v", expectedFlaggedPaths, comparison.FlaggedPaths)
	}

	expectedAddedPaths := []string{"added"}
	if !reflect.DeepEqual(comparison.AddedPaths, expectedAddedPaths) {
		t.Fatalf("expected AddedPaths %v; got %v", expectedAddedPaths, comparison.AddedPaths)
	}
}
