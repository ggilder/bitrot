package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func manyPathsComparison(n int) *ManifestComparison {
	oldEntries := map[string]ChecksumRecord{}
	for i := 0; i < n; i++ {
		oldEntries[pathN(i)] = ChecksumRecord{Checksum: pathN(i)}
	}
	oldManifest := &Manifest{Entries: oldEntries}
	newManifest := &Manifest{Entries: map[string]ChecksumRecord{}}
	return CompareManifests(oldManifest, newManifest)
}

func pathN(i int) string {
	return "path" + string(rune('a'+i))
}

func TestReportStringIsNeverTruncated(t *testing.T) {
	comparison := manyPathsComparison(5)
	report := NewComparisonReport(comparison)

	full := report.ReportString()
	assert.Contains(t, full, pathN(0))
	assert.Contains(t, full, pathN(4))
	assert.NotContains(t, full, "more (see full report)")
}

func TestTruncatedReportStringCapsPerSection(t *testing.T) {
	comparison := manyPathsComparison(5)
	report := NewComparisonReport(comparison)

	truncated := report.TruncatedReportString(2)
	assert.Contains(t, truncated, pathN(0))
	assert.Contains(t, truncated, pathN(1))
	assert.NotContains(t, truncated, pathN(2))
	assert.Contains(t, truncated, "... and 3 more (see full report)")
}

func TestTruncatedReportStringZeroMeansNoTruncation(t *testing.T) {
	comparison := manyPathsComparison(5)
	report := NewComparisonReport(comparison)

	assert.Equal(t, report.ReportString(), report.TruncatedReportString(0))
}

func TestTruncatedReportStringDoesNotTruncateBelowCount(t *testing.T) {
	comparison := manyPathsComparison(3)
	report := NewComparisonReport(comparison)

	truncated := report.TruncatedReportString(10)
	assert.NotContains(t, truncated, "more (see full report)")
}
