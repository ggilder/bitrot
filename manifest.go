package main

import (
	"golang.org/x/text/unicode/norm"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// ChecksumRecord stores checksum and metadata for a file.
type ChecksumRecord struct {
	Checksum string
	ModTime  time.Time
}

// Manifest of all files under a path.
type Manifest struct {
	Path      string
	CreatedAt time.Time
	Entries   map[string]ChecksumRecord
}

// FileError records a file that could not be read/hashed while generating a
// manifest. These are reported but don't abort the overall scan.
type FileError struct {
	Path  string
	Error error
}

// NewManifest generates a Manifest from a directory path, hashing files in
// parallel across workers goroutines. workers <= 0 defaults to the number of
// CPUs.
func NewManifest(path string, config *Config, workers int) (manifest *Manifest, errored []FileError, err error) {
	entries, errored, err := directoryChecksums(path, config, workers)
	if err != nil {
		return nil, nil, err
	}

	return &Manifest{
		Path:      path,
		CreatedAt: time.Now().UTC(),
		Entries:   entries,
	}, errored, nil
}

// Private functions

type checksumJob struct {
	relPath string
	absPath string
}

type checksumResult struct {
	relPath string
	record  ChecksumRecord
	err     error
}

func directoryChecksums(root string, config *Config, workers int) (map[string]ChecksumRecord, []FileError, error) {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	jobs := make(chan checksumJob)
	results := make(chan checksumResult)
	var workerGroup sync.WaitGroup

	for i := 0; i < workers; i++ {
		workerGroup.Add(1)
		go func() {
			defer workerGroup.Done()
			for job := range jobs {
				results <- hashJob(job)
			}
		}()
	}

	var walkErr error
	go func() {
		walkErr = filepath.Walk(root, func(entryPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			relPath, err := filepath.Rel(root, entryPath)
			if err != nil {
				return err
			}
			// Normalize Unicode combining characters
			relPath = norm.NFC.String(relPath)

			if config.isIgnoredPath(relPath) {
				if info.IsDir() {
					// Skip walking this directory
					return filepath.SkipDir
				}
				return nil
			}

			if info.Mode().IsRegular() {
				jobs <- checksumJob{relPath: relPath, absPath: entryPath}
			}

			return nil
		})
		close(jobs)
	}()

	go func() {
		workerGroup.Wait()
		close(results)
	}()

	records := map[string]ChecksumRecord{}
	var errored []FileError
	for result := range results {
		if result.err != nil {
			errored = append(errored, FileError{Path: result.relPath, Error: result.err})
			continue
		}
		records[result.relPath] = result.record
	}

	if walkErr != nil {
		return nil, nil, walkErr
	}

	return records, errored, nil
}

func hashJob(job checksumJob) checksumResult {
	info, err := os.Stat(job.absPath)
	if err != nil {
		return checksumResult{relPath: job.relPath, err: err}
	}

	checksum, err := hashFile(job.absPath)
	if err != nil {
		return checksumResult{relPath: job.relPath, err: err}
	}

	return checksumResult{
		relPath: job.relPath,
		record: ChecksumRecord{
			Checksum: checksum,
			ModTime:  info.ModTime().UTC(),
		},
	}
}
