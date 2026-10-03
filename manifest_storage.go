package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

const (
	manifestGlob         = "manifest-*.txt"
	manifestNameTemplate = "manifest-%s-%s.txt"
	// RFC3339 minus punctuation characters, better for filenames
	manifestNameTimeFormat      = "20060102T150405.000000000Z07:00"
	manifestStorageMetadataName = "bitrot_meta.json"
)

var manifestFilenameRe = regexp.MustCompile(`^manifest-([^-]+)-[^.]+\.txt$`)

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
	var buf bytes.Buffer
	if err := WriteManifest(&buf, manifest.Entries); err != nil {
		return err
	}
	content := buf.Bytes()

	manifestDir, err := m.addPath(manifest.Path)
	if err != nil {
		return err
	}
	filename := m.manifestFilename(manifest, content)
	manifestPath := filepath.Join(manifestDir, filename)

	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		err = ioutil.WriteFile(manifestPath, content, 0644)
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
	return m.readManifestFile(manifestPaths[0], path)
}

func (m *ManifestStorage) readManifestFile(filePath, forPath string) (*Manifest, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	entries, err := ReadManifest(f)
	if err != nil {
		return nil, err
	}

	createdAt, err := createdAtFromManifestFilename(filePath)
	if err != nil {
		return nil, err
	}

	return &Manifest{
		Path:      forPath,
		CreatedAt: createdAt,
		Entries:   entries,
	}, nil
}

func createdAtFromManifestFilename(filePath string) (time.Time, error) {
	base := filepath.Base(filePath)
	matches := manifestFilenameRe.FindStringSubmatch(base)
	if matches == nil {
		return time.Time{}, fmt.Errorf("unexpected manifest filename format: %s", base)
	}
	return time.Parse(manifestNameTimeFormat, matches[1])
}

func (m *ManifestStorage) parseMetadata(path string) (meta *ManifestStorageMetadata, err error) {
	// File already exists; check metadata
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &meta)
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
		data, err := json.Marshal(meta)
		if err != nil {
			return "", err
		}
		err = ioutil.WriteFile(metadataPath, data, 0644)
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
