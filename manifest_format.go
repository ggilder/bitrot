package main

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

// WriteManifest serializes manifest entries as one record per line, sorted by
// path. Each record is "<checksum> <mtime_unix_nano> <path_byte_length>\n"
// followed by exactly that many raw path bytes and a trailing newline. The
// path is length-prefixed rather than escaped, so arbitrary bytes (including
// newlines) in a path can never be confused with record structure.
func WriteManifest(w io.Writer, entries map[string]ChecksumRecord) error {
	paths := make([]string, 0, len(entries))
	for path := range entries {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	bw := bufio.NewWriter(w)
	for _, path := range paths {
		record := entries[path]
		pathBytes := []byte(path)
		if _, err := fmt.Fprintf(bw, "%s %d %d\n", record.Checksum, record.ModTime.UnixNano(), len(pathBytes)); err != nil {
			return err
		}
		if _, err := bw.Write(pathBytes); err != nil {
			return err
		}
		if _, err := bw.Write([]byte("\n")); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// ReadManifest parses the format written by WriteManifest.
func ReadManifest(r io.Reader) (map[string]ChecksumRecord, error) {
	entries := map[string]ChecksumRecord{}
	br := bufio.NewReader(r)

	for {
		header, err := br.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				return nil, err
			}
			if header == "" {
				return entries, nil
			}
		}
		header = strings.TrimRight(header, "\n")
		if header == "" {
			return entries, nil
		}

		parts := strings.SplitN(header, " ", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("malformed manifest header line: %q", header)
		}
		checksum := parts[0]
		modNano, convErr := strconv.ParseInt(parts[1], 10, 64)
		if convErr != nil {
			return nil, fmt.Errorf("malformed mod time in manifest header: %q", header)
		}
		pathLen, convErr := strconv.Atoi(parts[2])
		if convErr != nil {
			return nil, fmt.Errorf("malformed path length in manifest header: %q", header)
		}

		pathBytes := make([]byte, pathLen)
		if _, err := io.ReadFull(br, pathBytes); err != nil {
			return nil, fmt.Errorf("error reading path for manifest entry: %w", err)
		}
		trailing, err := br.ReadByte()
		if err != nil || trailing != '\n' {
			return nil, fmt.Errorf("expected newline after path for manifest entry %q", string(pathBytes))
		}

		entries[string(pathBytes)] = ChecksumRecord{
			Checksum: checksum,
			ModTime:  time.Unix(0, modNano).UTC(),
		}
	}
}
