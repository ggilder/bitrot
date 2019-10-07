package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
)

const (
	manifestGlob         = "manifest-*.json"
	manifestNameTemplate = "manifest-%s-%s.json"
	// RFC3339 minus punctuation characters, better for filenames
	manifestNameTimeFormat      = "20060102T150405Z07:00"
	manifestStorageMetadataName = "bitrot_meta.json"
)

type ManifestStorage struct {
	Path string
}

type ManifestStorageEntry struct {
	Path      string
	Id        string
	Manifests []*ManifestFileEntry
}

type ManifestFileEntry struct {
	SourcePath string
}

type ManifestStorageMetadata struct {
	Path string
}

func NewManifestStorage(path string) *ManifestStorage {
	return &ManifestStorage{Path: filepath.Clean(path)}
}

func (m *ManifestStorage) List() ([]*ManifestStorageEntry, error) {
	entries := []*ManifestStorageEntry{}
	entryMetadataFiles, _ := filepath.Glob(filepath.Join(m.Path, "*", manifestStorageMetadataName))
	for _, e := range entryMetadataFiles {
		meta, err := m.parseMetadata(e)
		if err != nil {
			return nil, err
		}

		entries = append(entries, &ManifestStorageEntry{
			Path: meta.Path,
			Id:   filepath.Base(filepath.Dir(e)),
		})
	}

	return entries, nil
}

func (m *ManifestStorage) AddManifest(manifest *Manifest) error {
	jsonBytes, err := json.Marshal(manifest)
	if err != nil {
		return err
	}

	manifestDir, err := m.addPath(manifest.Path)
	if err != nil {
		return err
	}
	filename := m.manifestFilename(manifest, jsonBytes)
	manifestPath := filepath.Join(manifestDir, filename)

	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		err = ioutil.WriteFile(manifestPath, jsonBytes, 0644)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("manifest file already exists at path %s", manifestPath)
	}

	return nil
}

func (m *ManifestStorage) LatestManifestForPath(path string) (*Manifest, error) {
	manifestDir, err := m.addPath(path)
	if err != nil {
		return nil, err
	}

	manifestPaths, err := filepath.Glob(filepath.Join(manifestDir, manifestGlob))
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(manifestPaths)))

	if len(manifestPaths) == 0 {
		return nil, nil
	}
	return m.readManifestFile(manifestPaths[0])
}

func (m *ManifestStorage) readManifestFile(path string) (*Manifest, error) {
	jsonBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest Manifest
	err = json.Unmarshal(jsonBytes, &manifest)
	if err != nil {
		return nil, err
	}
	return &manifest, nil
}

func (m *ManifestStorage) parseMetadata(path string) (meta *ManifestStorageMetadata, err error) {
	// File already exists; check metadata
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}
	err = json.Unmarshal(bytes, &meta)
	if err != nil {
		return
	}
	return
}

func (m *ManifestStorage) addPath(path string) (string, error) {
	// Using MkdirAll because it doesn't return an error when the path is already a directory
	manifestDir := m.storageForPath(path)
	err := os.MkdirAll(manifestDir, 0755)
	if err != nil {
		return "", err
	}

	// Make sure metadata exists in manifest storage directory
	metadataPath := filepath.Join(manifestDir, manifestStorageMetadataName)
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		// Write metadata
		meta := ManifestStorageMetadata{Path: path}
		bytes, err := json.Marshal(meta)
		if err != nil {
			return "", err
		}
		err = ioutil.WriteFile(metadataPath, bytes, 0644)
	} else {
		// File already exists; check metadata
		meta, err := m.parseMetadata(metadataPath)
		if err != nil {
			return "", err
		}
		if meta.Path != path {
			return "", fmt.Errorf("metadata in file %s does not match path %s", metadataPath, path)
		}
	}

	return manifestDir, nil
}

func (m *ManifestStorage) storageForPath(path string) string {
	pathHash := sha256.Sum256([]byte(path))
	pathHashHex := hex.EncodeToString(pathHash[:])
	return filepath.Join(m.Path, pathHashHex)
}

func (m *ManifestStorage) manifestFilename(manifest *Manifest, content []byte) string {
	return fmt.Sprintf(
		manifestNameTemplate,
		manifest.CreatedAt.Format(manifestNameTimeFormat),
		shortChecksum(content),
	)
}

// Short checksum suitable for a quick check on the manifest files
func shortChecksum(data []byte) string {
	checksum := crc32.ChecksumIEEE(data)
	checksumBytes := [4]byte{}
	binary.BigEndian.PutUint32(checksumBytes[:], checksum)
	return hex.EncodeToString(checksumBytes[:])
}
