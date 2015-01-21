package main

import (
	"crypto/sha1"
	"encoding/hex"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// ChecksumRecord stores checksum and metadata for a file.
type ChecksumRecord struct {
	Checksum string    `json:"checksum"`
	ModTime  time.Time `json:"mod_time"`
}

// Manifest of all files under a path.
type Manifest struct {
	Path      string                    `json:"path"`
	CreatedAt time.Time                 `json:"created_at"`
	Entries   map[string]ChecksumRecord `json:"entries"`
}

// ManifestComparison of two Manifests, showing paths that have been deleted,
// added, modified, or flagged for suspicious checksum changes.
type ManifestComparison struct {
	DeletedPaths  []string
	AddedPaths    []string
	ModifiedPaths []string
	FlaggedPaths  []string
}

// ChecksumRecordForFile generates a ChecksumRecord from a file path.
func ChecksumRecordForFile(file string) ChecksumRecord {
	fi, err := os.Stat(file)
	check(err)

	sum := generateChecksum(file)
	return ChecksumRecord{
		Checksum: sum,
		ModTime:  fi.ModTime().UTC(),
	}
}

// ManifestForPath generates a Manifest from a directory path.
func ManifestForPath(path string, config *Config) Manifest {
	return Manifest{
		Path:      path,
		CreatedAt: time.Now().UTC(),
		Entries:   directoryChecksums(path, config),
	}
}

// CompareManifests generates a comparison between new and old Manifests.
func CompareManifests(oldManifest, newManifest Manifest) (comparison ManifestComparison) {
	checkAddedPaths(&oldManifest, &newManifest, &comparison)
	checkChangedPaths(&oldManifest, &newManifest, &comparison)
	return comparison
}

// Private functions

func generateChecksum(file string) string {
	data, err := ioutil.ReadFile(file)
	check(err)

	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:])
}

func directoryChecksums(path string, config *Config) map[string]ChecksumRecord {
	records := map[string]ChecksumRecord{}
	filepath.Walk(path, func(entryPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if config.isIgnoredPath(entryPath) {
			return nil
		}

		if info.Mode().IsRegular() {
			var relPath string
			relPath, err = filepath.Rel(path, entryPath)
			check(err)
			records[relPath] = ChecksumRecord{
				Checksum: generateChecksum(entryPath),
				ModTime:  info.ModTime().UTC(),
			}
		}

		return nil
	})
	return records
}

func checkAddedPaths(oldManifest, newManifest *Manifest, comparison *ManifestComparison) {
	for path := range newManifest.Entries {
		_, oldEntryPresent := oldManifest.Entries[path]
		if !oldEntryPresent {
			comparison.AddedPaths = append(comparison.AddedPaths, path)
		}
	}
}

func checkChangedPaths(oldManifest, newManifest *Manifest, comparison *ManifestComparison) {
	for path, oldEntry := range oldManifest.Entries {
		newEntry, newEntryPresent := newManifest.Entries[path]
		if newEntryPresent {
			checkModifiedPath(path, &oldEntry, &newEntry, comparison)
		} else {
			comparison.DeletedPaths = append(comparison.DeletedPaths, path)
		}
	}
}

func checkModifiedPath(path string, oldEntry, newEntry *ChecksumRecord, comparison *ManifestComparison) {
	if newEntry.Checksum != oldEntry.Checksum {
		if newEntry.ModTime != oldEntry.ModTime {
			comparison.ModifiedPaths = append(comparison.ModifiedPaths, path)
		} else {
			comparison.FlaggedPaths = append(comparison.FlaggedPaths, path)
		}
	}
}
