package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func setupTestManifests() (oldManifest *Manifest, newManifest *Manifest) {
	newCreatedAt := time.Now()
	newModTime := time.Now()
	oldCreatedAt := newCreatedAt.Add(-24 * time.Hour)
	oldModTime := newModTime.Add(-24 * time.Hour)
	oldManifest = &Manifest{
		Path:      "/old/stuff",
		CreatedAt: oldCreatedAt,
		Entries: map[string]ChecksumRecord{
			"silently_corrupted": {Checksum: "asdf", ModTime: oldModTime},
			"not_changed":        {Checksum: "zxcv", ModTime: oldModTime},
			"modified":           {Checksum: "qwer", ModTime: oldModTime},
			"touched":            {Checksum: "olkm", ModTime: oldModTime},
			"deleted":            {Checksum: "jklh", ModTime: oldModTime},
			"renamedOld":         {Checksum: "xxxx", ModTime: oldModTime},
		},
	}

	newManifest = &Manifest{
		Path:      "/new/thing",
		CreatedAt: newCreatedAt,
		Entries: map[string]ChecksumRecord{
			"silently_corrupted": {Checksum: "zzzz", ModTime: oldModTime},
			"not_changed":        {Checksum: "zxcv", ModTime: oldModTime},
			"modified":           {Checksum: "tyui", ModTime: newModTime},
			"touched":            {Checksum: "olkm", ModTime: newModTime},
			"added":              {Checksum: "bnmv", ModTime: newModTime},
			"renamedNew":         {Checksum: "xxxx", ModTime: oldModTime},
		},
	}
	return
}

func TestManifestComparison(t *testing.T) {
	oldManifest, newManifest := setupTestManifests()
	comparison := CompareManifests(oldManifest, newManifest)

	assert.ElementsMatch(t, comparison.UnchangedPaths, []string{"not_changed", "touched"})
	assert.ElementsMatch(t, comparison.DeletedPaths, []string{"deleted"})
	assert.ElementsMatch(t, comparison.ModifiedPaths, []string{"modified"})
	assert.ElementsMatch(t, comparison.FlaggedPaths, []string{"silently_corrupted"})
	assert.ElementsMatch(t, comparison.AddedPaths, []string{"added"})
	assert.ElementsMatch(t, comparison.RenamedPaths, []RenamedPath{{OldPath: "renamedOld", NewPath: "renamedNew"}})
}

func TestSuccess(t *testing.T) {
	oldManifest, newManifest := setupTestManifests()
	comparison := CompareManifests(oldManifest, newManifest)
	assert.False(t, comparison.Success())

	comparison.FlaggedPaths = []string{}
	assert.True(t, comparison.Success())
}

func TestTotalChecked(t *testing.T) {
	oldManifest, newManifest := setupTestManifests()
	comparison := CompareManifests(oldManifest, newManifest)
	assert.Equal(t, 7, comparison.TotalChecked())
}

func TestManifestComparisonMatchesDuplicateContentOneToOne(t *testing.T) {
	// Two old paths and two new paths share the same checksum ("dup"); each
	// old path should be matched to a distinct new path, not all-to-all.
	oldManifest := &Manifest{
		Entries: map[string]ChecksumRecord{
			"old1": {Checksum: "dup"},
			"old2": {Checksum: "dup"},
		},
	}
	newManifest := &Manifest{
		Entries: map[string]ChecksumRecord{
			"new1": {Checksum: "dup"},
			"new2": {Checksum: "dup"},
		},
	}

	comparison := CompareManifests(oldManifest, newManifest)

	assert.Empty(t, comparison.AddedPaths)
	assert.Empty(t, comparison.DeletedPaths)
	assert.Len(t, comparison.RenamedPaths, 2)

	matchedOld := map[string]bool{}
	matchedNew := map[string]bool{}
	for _, renamed := range comparison.RenamedPaths {
		matchedOld[renamed.OldPath] = true
		matchedNew[renamed.NewPath] = true
	}
	assert.True(t, matchedOld["old1"])
	assert.True(t, matchedOld["old2"])
	assert.True(t, matchedNew["new1"])
	assert.True(t, matchedNew["new2"])
}

func TestCaching(t *testing.T) {
	oldManifest, newManifest := setupTestManifests()
	comparison := &ManifestComparison{oldManifest: oldManifest, newManifest: newManifest, complete: true}
	comparison.compare()
	assert.Empty(t, comparison.UnchangedPaths)
	assert.Empty(t, comparison.DeletedPaths)
	assert.Empty(t, comparison.ModifiedPaths)
	assert.Empty(t, comparison.FlaggedPaths)
	assert.Empty(t, comparison.AddedPaths)
	assert.Empty(t, comparison.RenamedPaths)
}
