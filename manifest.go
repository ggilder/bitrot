package main

import (
	"crypto/sha1"
	"encoding/hex"
	"golang.org/x/text/unicode/norm"
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

// NewManifest generates a Manifest from a directory path.
func NewManifest(path string, config *Config) (*Manifest, error) {
	entries, err := directoryChecksums(path, config)
	if err != nil {
		return nil, err
	}

	return &Manifest{
		Path:      path,
		CreatedAt: time.Now().UTC(),
		Entries:   entries,
	}, nil
}

// Private functions

func generateChecksum(file string) (string, error) {
	// TODO: experiment with varying buffer to determine optimal size
	bufferSize := 10 * 1024 * 1024 // 10MiB buffer
	reader, err := newSha1Reader(file, bufferSize)
	if err != nil {
		return "", err
	}
	sum, err := reader.SHA1Sum()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sum), nil
}

func checksumHexString(data *[]byte) string {
	sum := sha1.Sum(*data)
	return hex.EncodeToString(sum[:])
}

func directoryChecksums(path string, config *Config) (map[string]ChecksumRecord, error) {
	records := map[string]ChecksumRecord{}
	err := filepath.Walk(path, func(entryPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if config.isIgnoredPath(entryPath) {
			return nil
		}

		if info.Mode().IsRegular() {
			var relPath string
			relPath, err = filepath.Rel(path, entryPath)
			if err != nil {
				return err
			}
			// Normalize Unicode combining characters
			relPath = norm.NFC.String(relPath)
			checksum, err := generateChecksum(entryPath)
			if err != nil {
				return err
			}
			records[relPath] = ChecksumRecord{
				Checksum: checksum,
				ModTime:  info.ModTime().UTC(),
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}
