package main

// ManifestComparison of two Manifests, showing paths that have been deleted,
// added, renamed, modified, or flagged for suspicious checksum changes
// (indicating possible corruption).
type ManifestComparison struct {
	UnchangedPaths []string
	DeletedPaths   []string
	AddedPaths     []string
	RenamedPaths   []RenamedPath
	ModifiedPaths  []string
	FlaggedPaths   []string
	oldManifest    *Manifest
	newManifest    *Manifest
	complete       bool
}

// RenamedPath tracks a path that has been moved/renamed but has the same
// content.
type RenamedPath struct {
	OldPath string
	NewPath string
}

// CompareManifests generates a comparison between new and old Manifests.
func CompareManifests(oldManifest, newManifest *Manifest) *ManifestComparison {
	comparison := &ManifestComparison{oldManifest: oldManifest, newManifest: newManifest}
	comparison.compare()
	return comparison
}

func (comp *ManifestComparison) compare() {
	// Don't rerun
	if comp.complete {
		// TODO test this behavior
		return
	}

	// First look for paths added in new
	for path := range comp.newManifest.Entries {
		_, oldEntryPresent := comp.oldManifest.Entries[path]
		if !oldEntryPresent {
			comp.AddedPaths = append(comp.AddedPaths, path)
		}
	}

	// Then look for modifications, deletions, renames, or corruptions of files from old to new
	for path, oldEntry := range comp.oldManifest.Entries {
		// Handle a matching path entry in new manifest
		if comp.handleEntry(path, &oldEntry) {
			continue
		}

		// Handle a renamed path in new manifest
		if comp.handleRenamedEntry(path, &oldEntry) {
			continue
		}

		// If no matching or renamed entry in new manifest, entry was deleted
		comp.DeletedPaths = append(comp.DeletedPaths, path)
	}

	comp.complete = true
}

func (comp *ManifestComparison) handleEntry(path string, oldEntry *ChecksumRecord) bool {
	newEntry, newEntryPresent := comp.newManifest.Entries[path]
	if !newEntryPresent {
		return false
	}

	if newEntry.Checksum == oldEntry.Checksum {
		comp.UnchangedPaths = append(comp.UnchangedPaths, path)
	} else {
		if newEntry.ModTime != oldEntry.ModTime {
			// Content change plus mod time change = intended modification
			comp.ModifiedPaths = append(comp.ModifiedPaths, path)
		} else {
			// Content change with no mod time change = possible corruption
			comp.FlaggedPaths = append(comp.FlaggedPaths, path)
		}
	}

	return true
}

func (comp *ManifestComparison) handleRenamedEntry(path string, oldEntry *ChecksumRecord) bool {
	newPath := comp.findRenamedPathByChecksum(oldEntry.Checksum)
	if newPath == "" {
		return false
	}

	comp.RenamedPaths = append(comp.RenamedPaths, RenamedPath{OldPath: path, NewPath: newPath})

	// Remove from added paths
	for idx, path := range comp.AddedPaths {
		if path == newPath {
			comp.AddedPaths = append(comp.AddedPaths[:idx], comp.AddedPaths[idx+1:]...)
			break
		}
	}

	return true
}

func (comp *ManifestComparison) findRenamedPathByChecksum(checksum string) string {
	for _, newPath := range comp.AddedPaths {
		if comp.newManifest.Entries[newPath].Checksum == checksum {
			return newPath
		}
	}
	return ""
}
