package bitrot

import (
	"crypto/sha1"
	"encoding/hex"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	ExcludedFiles []string
}

type ChecksumRecord struct {
	Checksum string    `json:"checksum"`
	ModTime  time.Time `json:"mod_time"`
}

type DirectoryManifest struct {
	Path      string                    `json:"path"`
	CreatedAt time.Time                 `json:"created_at"`
	Entries   map[string]ChecksumRecord `json:"entries"`
}

type ManifestComparison struct {
	DeletedPaths  []string
	AddedPaths    []string
	ModifiedPaths []string
	FlaggedPaths  []string
}

func FileChecksum(file string) ChecksumRecord {
	fi, err := os.Stat(file)
	check(err)

	sum := generateChecksum(file)
	return ChecksumRecord{
		Checksum: sum,
		ModTime:  fi.ModTime(),
	}
}

func GenerateDirectoryManifest(path string, config *Config) DirectoryManifest {
	return DirectoryManifest{
		Path:      path,
		CreatedAt: time.Now(),
		Entries:   directoryChecksums(path, config),
	}
}

func CompareManifests(oldManifest, newManifest DirectoryManifest) (comparison ManifestComparison) {
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

func isIgnoredPath(path string, ignored *[]string) bool {
	parts := strings.Split(path, string(filepath.Separator))
	for _, part := range parts {
		for _, ignoredName := range *ignored {
			if part == ignoredName {
				return true
			}
		}
	}
	return false
}

func directoryChecksums(path string, config *Config) map[string]ChecksumRecord {
	records := map[string]ChecksumRecord{}
	filepath.Walk(path, func(entryPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if isIgnoredPath(entryPath, &config.ExcludedFiles) {
			return nil
		}

		if info.Mode().IsRegular() {
			var relPath string
			relPath, err = filepath.Rel(path, entryPath)
			check(err)
			records[relPath] = ChecksumRecord{
				Checksum: generateChecksum(entryPath),
				ModTime:  info.ModTime(),
			}
		}

		return nil
	})
	return records
}

func checkAddedPaths(oldManifest, newManifest *DirectoryManifest, comparison *ManifestComparison) {
	for path, _ := range newManifest.Entries {
		_, oldEntryPresent := oldManifest.Entries[path]
		if !oldEntryPresent {
			comparison.AddedPaths = append(comparison.AddedPaths, path)
		}
	}
}

func checkChangedPaths(oldManifest, newManifest *DirectoryManifest, comparison *ManifestComparison) {
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

func check(e error) {
	if e != nil {
		panic(e)
	}
}
