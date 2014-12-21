package checksum_generator

import (
	"crypto/sha1"
	"encoding/hex"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

type ChecksumRecord struct {
	Checksum string    `json:"checksum"`
	ModTime  time.Time `json:"mod_time"`
}

type DirectoryManifest struct {
	Path      string                    `json:"path"`
	CreatedAt time.Time                 `json:"created_at"`
	Entries   map[string]ChecksumRecord `json:"entries"`
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

func GenerateDirectoryManifest(path string) DirectoryManifest {
	return DirectoryManifest{
		Path:      path,
		CreatedAt: time.Now(),
		Entries:   directoryChecksums(path),
	}
}

// Private functions

func generateChecksum(file string) string {
	data, err := ioutil.ReadFile(file)
	check(err)

	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:])
}

func directoryChecksums(path string) map[string]ChecksumRecord {
	records := map[string]ChecksumRecord{}
	filepath.Walk(path, func(entryPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		fi, err := os.Stat(entryPath)
		check(err)

		if fi.Mode().IsRegular() {
			var relPath string
			relPath, err = filepath.Rel(path, entryPath)
			check(err)
			records[relPath] = FileChecksum(entryPath)
		}

		return nil
	})
	return records
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
