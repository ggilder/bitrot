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
	return report.SummaryString() + "\n\n\n" + report.DetailString()
}

func (report *ComparisonReport) SummaryString() string {
	s := ""
	if report.mc.Success() {
		s += "SUCCESS"
	} else {
		s += "FAILURE"
	}
	s += "\n\n"

	s += fmt.Sprintf("%d files compared.\n\n", report.mc.TotalChecked())

	s += report.summaryLine("Unchanged", report.mc.UnchangedPaths)
	s += report.summaryLine("Added", report.mc.AddedPaths)
	s += report.summaryLine("Deleted", report.mc.DeletedPaths)
	s += fmt.Sprintf("Renamed paths: %d\n", len(report.mc.RenamedPaths))
	s += report.summaryLine("Modified", report.mc.ModifiedPaths)
	s += report.summaryLine("Flagged", report.mc.FlaggedPaths)

	return s
}

func (report *ComparisonReport) DetailString() string {
	return report.unchangedSection() +
		report.pathSection("Added", report.mc.AddedPaths) +
		report.pathSection("Deleted", report.mc.DeletedPaths) +
		report.renamedSection() +
		report.pathSection("Modified", report.mc.ModifiedPaths) +
		report.pathSection("Flagged", report.mc.FlaggedPaths)
}

func (report *ComparisonReport) summaryLine(description string, paths []string) string {
	count := len(paths)
	return fmt.Sprintf("%s paths: %d\n", description, count)
}

func (report *ComparisonReport) pathSection(description string, paths []string) string {
	s := report.summaryLine(description, paths)
	for _, path := range paths {
		s += fmt.Sprintf("    %s\n", path)
	}
	return s
}

func (report *ComparisonReport) unchangedSection() string {
	return report.summaryLine("Unchanged", report.mc.UnchangedPaths)
}

func (report *ComparisonReport) renamedSection() string {
	entries := report.mc.RenamedPaths
	s := ""
	count := len(entries)
	s += fmt.Sprintf("Renamed paths: %d\n", count)
	for _, entry := range entries {
		s += fmt.Sprintf("    %s -> %s\n", entry.OldPath, entry.NewPath)
	}
	return s
}
