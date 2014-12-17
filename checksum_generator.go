package checksum_generator

import (
	"crypto/sha1"
	"encoding/hex"
	"io/ioutil"
	"os"
	"time"
)

type ChecksumRecord struct {
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
	return ChecksumRecord{hex.EncodeToString(sum[:]), fi.ModTime()}
}
