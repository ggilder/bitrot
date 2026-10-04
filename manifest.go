package main

import (
	"golang.org/x/text/unicode/norm"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
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
// CPUs. progress is purely cosmetic: it only affects what's reported via
// progress, not the resulting Manifest.
func NewManifest(path string, config *Config, workers int, progress ProgressFunc) (manifest *Manifest, errored []FileError, err error) {
	entries, errored, err := directoryChecksums(path, config, workers, progress)
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
	size    int64
	err     error
}

func directoryChecksums(root string, config *Config, workers int, progress ProgressFunc) (map[string]ChecksumRecord, []FileError, error) {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	// jobs is buffered so the walker (cheap stat/readdir calls) can race
	// ahead of the hash workers (slow - full content reads) rather than
	// blocking on every single handoff. An unbuffered channel would throttle
	// the walk to roughly hashing's pace, defeating the point of tracking
	// "discovered" separately from "scanned": it would never meaningfully
	// lead, and the live total would just track completed work instead of
	// becoming exact as soon as the walk itself finishes. Comfortably covers
	// the ~900K files on /Volumes/plethora today with room to grow; the
	// buffer's backing array costs ~32MB at this size, trivial here.
	const jobQueueCapacity = 1_000_000
	jobs := make(chan checksumJob, jobQueueCapacity)
	results := make(chan checksumResult)
	var workerGroup sync.WaitGroup
	// Incremented by the walker at discovery time (before hashing), not by
	// the result loop at completion time - so it runs ahead of "scanned"
	// and becomes an exact total as soon as the walk finishes, rather than
	// needing a previous manifest to seed an estimate from.
	var discovered atomic.Int64
	var walkComplete atomic.Bool

	// 4MiB, matching dropbox-sync-verifier's block size - allocated once
	// per worker and reused across every file that worker hashes, rather
	// than per file.
	const hashBufferSize = 4 * 1024 * 1024

	for i := 0; i < workers; i++ {
		workerGroup.Add(1)
		go func() {
			defer workerGroup.Done()
			buf := make([]byte, hashBufferSize)
			for job := range jobs {
				results <- hashJob(job, buf)
			}
		}()
	}

	var walkErr error
	go func() {
		walkErr = filepath.Walk(root, func(entryPath string, info os.FileInfo, err error) error {
			// Compute relPath and check exclusion before looking at err: an
			// unreadable excluded directory (e.g. macOS's .DocumentRevisions-V100,
			// which is execute-only) should still be skipped rather than
			// aborting the walk, since filepath.Walk surfaces a directory's
			// own read error on the very call where we'd otherwise decide to
			// skip it.
			relPath, relErr := filepath.Rel(root, entryPath)
			if relErr != nil {
				return relErr
			}
			// Normalize Unicode combining characters
			relPath = norm.NFC.String(relPath)

			if config.isIgnoredPath(relPath) {
				if info != nil && info.IsDir() {
					// Skip walking this directory
					return filepath.SkipDir
				}
				return nil
			}

			if err != nil {
				// The scan root itself being unreadable is fatal; a
				// permission problem elsewhere in the tree is not - report
				// it and keep going rather than abort an otherwise-fine,
				// possibly multi-hour scan over one bad subtree.
				if relPath == "." {
					return err
				}
				discovered.Add(1)
				results <- checksumResult{relPath: relPath, err: err}
				if info != nil && info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if info.Mode().IsRegular() {
				discovered.Add(1)
				jobs <- checksumJob{relPath: relPath, absPath: entryPath}
			}

			return nil
		})
		walkComplete.Store(true)
		close(jobs)
	}()

	go func() {
		workerGroup.Wait()
		close(results)
	}()

	records := map[string]ChecksumRecord{}
	var errored []FileError
	var scanned int
	var bytesHashed int64
	start := time.Now()
	lastReported := time.Time{}
	const progressInterval = 200 * time.Millisecond

	for result := range results {
		scanned++
		if result.err != nil {
			errored = append(errored, FileError{Path: result.relPath, Error: result.err})
		} else {
			records[result.relPath] = result.record
			bytesHashed += result.size
		}

		if progress != nil && time.Since(lastReported) >= progressInterval {
			progress(ProgressStats{
				Scanned:        scanned,
				EstimatedTotal: int(discovered.Load()),
				TotalIsExact:   walkComplete.Load(),
				BytesHashed:    bytesHashed,
				Errored:        len(errored),
				Elapsed:        time.Since(start),
			})
			lastReported = time.Now()
		}
	}

	if progress != nil {
		progress(ProgressStats{
			Scanned:        scanned,
			EstimatedTotal: int(discovered.Load()),
			TotalIsExact:   walkComplete.Load(),
			BytesHashed:    bytesHashed,
			Errored:        len(errored),
			Elapsed:        time.Since(start),
		})
	}

	if walkErr != nil {
		return nil, nil, walkErr
	}

	return records, errored, nil
}

func hashJob(job checksumJob, buf []byte) checksumResult {
	info, err := os.Stat(job.absPath)
	if err != nil {
		return checksumResult{relPath: job.relPath, err: err}
	}

	checksum, err := hashFile(job.absPath, buf)
	if err != nil {
		return checksumResult{relPath: job.relPath, err: err}
	}

	return checksumResult{
		relPath: job.relPath,
		size:    info.Size(),
		record: ChecksumRecord{
			Checksum: checksum,
			ModTime:  info.ModTime().UTC(),
		},
	}
}
