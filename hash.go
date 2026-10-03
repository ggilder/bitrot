package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// hashFile computes the hex-encoded SHA-256 checksum of a file's contents.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
