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
	path     string
	checksum string
	modTime  time.Time
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func FileChecksum(file string) ChecksumRecord {
	var fi os.FileInfo
	data, err := ioutil.ReadFile(file)
	check(err)

	fi, err = os.Stat(file)
	check(err)

	sum := sha1.Sum(data)
	return ChecksumRecord{file, hex.EncodeToString(sum[:]), fi.ModTime()}
}

func DirectoryChecksums(path string) map[string]ChecksumRecord {
	records := map[string]ChecksumRecord{}
	filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		fi, err := os.Stat(path)
		check(err)

		if fi.Mode().IsRegular() {
			records[path] = FileChecksum(path)
		}

		return nil
	})
	return records
}
