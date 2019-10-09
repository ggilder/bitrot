package main

import (
	"reflect"
	"testing"
	"time"
)

// TODO refactor tests to use testify/assert library like `bitrot_test.go`
func TestManifestComparison(t *testing.T) {
	newCreatedAt := time.Now()
	newModTime := time.Now()
	oldCreatedAt := newCreatedAt.Add(-24 * time.Hour)
	oldModTime := newModTime.Add(-24 * time.Hour)
	oldManifest := Manifest{
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

	newManifest := Manifest{
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

	comparison := CompareManifests(&oldManifest, &newManifest)

	expectedDeletedPaths := []string{"deleted"}
	if !reflect.DeepEqual(comparison.DeletedPaths, expectedDeletedPaths) {
		t.Fatalf("expected DeletedPaths %v; got %v", expectedDeletedPaths, comparison.DeletedPaths)
	}

	expectedModifiedPaths := []string{"modified"}
	if !reflect.DeepEqual(comparison.ModifiedPaths, expectedModifiedPaths) {
		t.Fatalf("expected ModifiedPaths %v; got %v", expectedModifiedPaths, comparison.ModifiedPaths)
	}

	expectedFlaggedPaths := []string{"silently_corrupted"}
	if !reflect.DeepEqual(comparison.FlaggedPaths, expectedFlaggedPaths) {
		t.Fatalf("expected FlaggedPaths %v; got %v", expectedFlaggedPaths, comparison.FlaggedPaths)
	}

	expectedAddedPaths := []string{"added"}
	if !reflect.DeepEqual(comparison.AddedPaths, expectedAddedPaths) {
		t.Fatalf("expected AddedPaths %v; got %v", expectedAddedPaths, comparison.AddedPaths)
	}

	expectedRenamedPaths := []RenamedPath{{OldPath: "renamedOld", NewPath: "renamedNew"}}
	if !reflect.DeepEqual(comparison.RenamedPaths, expectedRenamedPaths) {
		t.Fatalf("expected RenamedPaths %v; got %v", expectedRenamedPaths, comparison.RenamedPaths)
	}
}
