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

func newSha1Reader(path string, bufferSize int) (*sha1Reader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &sha1Reader{
		hash:       sha1.New(),
		reader:     file,
		bufferSize: bufferSize,
	}, nil
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
}
