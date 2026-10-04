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

func (comp *ManifestComparison) Success() bool {
	return len(comp.FlaggedPaths) == 0
}

func (comp *ManifestComparison) TotalChecked() int {
	return len(comp.UnchangedPaths) +
		len(comp.DeletedPaths) +
		len(comp.AddedPaths) +
		len(comp.RenamedPaths) +
		len(comp.ModifiedPaths) +
		len(comp.FlaggedPaths)
}

func (comp *ManifestComparison) compare() {
	// Don't rerun
	if comp.complete {
		return
	}

	// Index paths added in new by checksum, so renamed-file matching below is
	// O(1) per lookup instead of a linear scan over every added path.
	addedByChecksum := map[string][]string{}
	for path, newEntry := range comp.newManifest.Entries {
		if _, oldEntryPresent := comp.oldManifest.Entries[path]; !oldEntryPresent {
			addedByChecksum[newEntry.Checksum] = append(addedByChecksum[newEntry.Checksum], path)
		}
	}

	// Look for modifications, deletions, renames, or corruptions of files from old to new
	for path, oldEntry := range comp.oldManifest.Entries {
		// Handle a matching path entry in new manifest
		if comp.handleEntry(path, &oldEntry) {
			continue
		}

		// Handle a renamed path in new manifest
		if comp.handleRenamedEntry(path, &oldEntry, addedByChecksum) {
			continue
		}

		// If no matching or renamed entry in new manifest, entry was deleted
		comp.DeletedPaths = append(comp.DeletedPaths, path)
	}

	// Whatever's left in addedByChecksum wasn't claimed as a rename target
	for _, paths := range addedByChecksum {
		comp.AddedPaths = append(comp.AddedPaths, paths...)
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

func (comp *ManifestComparison) handleRenamedEntry(path string, oldEntry *ChecksumRecord, addedByChecksum map[string][]string) bool {
	candidates := addedByChecksum[oldEntry.Checksum]
	if len(candidates) == 0 {
		return false
	}

	newPath := candidates[0]
	if len(candidates) == 1 {
		delete(addedByChecksum, oldEntry.Checksum)
	} else {
		addedByChecksum[oldEntry.Checksum] = candidates[1:]
	}

	comp.RenamedPaths = append(comp.RenamedPaths, RenamedPath{OldPath: path, NewPath: newPath})

	return true
}
