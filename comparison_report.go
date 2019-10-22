package main

import (
	"fmt"
)

// ComparisonReport handles summarizing and formatting the results of a manifest comparison.
type ComparisonReport struct {
	mc *ManifestComparison
}

func NewComparisonReport(comparison *ManifestComparison) *ComparisonReport {
	report := &ComparisonReport{mc: comparison}
	return report
}

func (report *ComparisonReport) ReportString() string {
	return report.unchangedSection() +
		report.pathSection("Added", report.mc.AddedPaths) +
		report.pathSection("Deleted", report.mc.DeletedPaths) +
		report.renamedSection() +
		report.pathSection("Modified", report.mc.ModifiedPaths) +
		report.pathSection("Flagged", report.mc.FlaggedPaths)
}

func (report *ComparisonReport) pathSection(description string, paths []string) string {
	s := ""
	count := len(paths)
	if count > 0 {
		s += fmt.Sprintf("%s paths:\n", description)
		for _, path := range paths {
			s += fmt.Sprintf("    %s\n", path)
		}
	} else {
		s += fmt.Sprintf("%s paths: none.\n", description)
	}
	return s
}

func (report *ComparisonReport) unchangedSection() string {
	count := len(report.mc.UnchangedPaths)
	return fmt.Sprintf("Unchanged paths: %d\n", count)
}

func (report *ComparisonReport) renamedSection() string {
	entries := report.mc.RenamedPaths
	s := ""
	count := len(entries)
	if count > 0 {
		s += fmt.Sprint("Renamed paths:\n")
		for _, entry := range entries {
			s += fmt.Sprintf("    %s -> %s\n", entry.OldPath, entry.NewPath)
		}
	} else {
		s += fmt.Sprint("Renamed paths: none.\n")
	}
	return s
}
