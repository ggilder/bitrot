package main

import (
	"crypto/sha1"
	"hash"
	"io"
	"os"
)

type sha1Reader struct {
	hash       hash.Hash
	reader     io.ReadSeeker
	bufferSize int
}

func newSha1Reader(path string, bufferSize int) *sha1Reader {
	file, err := os.Open(path)
	check(err)
	return &sha1Reader{
		hash:       sha1.New(),
		reader:     file,
		bufferSize: bufferSize,
	}
}

func (r *sha1Reader) SHA1Sum() ([]byte, error) {
	_, err := r.reader.Seek(0, 0)
	if err != nil {
		return []byte{}, err
	}
	err = r.readAll()
	if err != nil {
		return []byte{}, err
	}
	return r.hash.Sum(nil), nil
}

func (r *sha1Reader) readAll() (err error) {
	for {
		b := make([]byte, r.bufferSize, r.bufferSize)
		n, err := r.reader.Read(b)
		if err != nil && err != io.EOF {
			return err
		}
		r.hash.Write(b[:n])
		if err == io.EOF {
			return nil
		}
	}
	return nil
}
