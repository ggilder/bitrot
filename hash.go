package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// hashFile computes the hex-encoded SHA-256 checksum of a file's contents,
// reading through buf. buf is caller-provided (and reused across many
// files, one per worker) rather than left to io.Copy's own default: on
// Darwin, *os.File implements io.WriterTo, which io.Copy always prefers,
// and that path ignores any buffer passed in and falls back to a fresh
// 32KiB buffer per call regardless - reading many small chunks per file
// instead of a few large ones, which matters on a seek-heavy spinning
// array. A plain read loop sidesteps that entirely.
func hashFile(path string, buf []byte) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n]) // hash.Hash.Write never returns an error
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
