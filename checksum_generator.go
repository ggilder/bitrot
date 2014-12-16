package checksum_generator

import (
	"crypto/sha1"
	"encoding/hex"
	"io/ioutil"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func FileChecksum(file string) string {
	data, err := ioutil.ReadFile(file)
	check(err)
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:])
}
