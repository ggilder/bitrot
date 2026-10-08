package main

import (
	"fmt"
	"sort"
	"strings"
)

// ComparisonReport handles summarizing and formatting the results of a manifest comparison.
type ComparisonReport struct {
	mc *ManifestComparison
}

func NewComparisonReport(comparison *ManifestComparison) *ComparisonReport {
	report := &ComparisonReport{mc: comparison}
	return report
}

// ReportString is the full, untruncated report.
func (report *ComparisonReport) ReportString() string {
	return report.SummaryString() + "\n\n\n" + report.detailString(0)
}

// TruncatedReportString caps the number of paths listed per section at
// maxPerSection (0 means no truncation, same as ReportString). Intended for
// a destination like an email body where a large reorganization could
// otherwise produce an unreadable wall of paths; the full report should
// still be written somewhere untruncated.
func (report *ComparisonReport) TruncatedReportString(maxPerSection int) string {
	return report.SummaryString() + "\n\n\n" + report.detailString(maxPerSection)
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

// DetailString is the full, untruncated detail section.
func (report *ComparisonReport) DetailString() string {
	return report.detailString(0)
}

func (report *ComparisonReport) detailString(maxPerSection int) string {
	sections := []string{
		report.unchangedSection(),
		report.pathSection("Added", report.mc.AddedPaths, maxPerSection),
		report.pathSection("Deleted", report.mc.DeletedPaths, maxPerSection),
		report.renamedSection(maxPerSection),
		report.pathSection("Modified", report.mc.ModifiedPaths, maxPerSection),
		report.pathSection("Flagged", report.mc.FlaggedPaths, maxPerSection),
	}
	return strings.Join(sections, "\n")
}

func (report *ComparisonReport) summaryLine(description string, paths []string) string {
	count := len(paths)
	return fmt.Sprintf("%s paths: %d\n", description, count)
}

func (report *ComparisonReport) pathSection(description string, paths []string, maxShown int) string {
	s := report.summaryLine(description, paths)
	shown := append([]string(nil), paths...)
	sort.Strings(shown)
	omitted := 0
	if maxShown > 0 && len(shown) > maxShown {
		shown = shown[:maxShown]
		omitted = len(paths) - maxShown
	}
	for _, path := range shown {
		s += fmt.Sprintf("    %s\n", path)
	}
	if omitted > 0 {
		s += fmt.Sprintf("    ... and %d more (see full report)\n", omitted)
	}
	return s
}

func (report *ComparisonReport) unchangedSection() string {
	return report.summaryLine("Unchanged", report.mc.UnchangedPaths)
}

func (report *ComparisonReport) renamedSection(maxShown int) string {
	entries := report.mc.RenamedPaths
	s := ""
	count := len(entries)
	s += fmt.Sprintf("Renamed paths: %d\n", count)
	shown := append([]RenamedPath(nil), entries...)
	sort.Slice(shown, func(i, j int) bool { return shown[i].OldPath < shown[j].OldPath })
	omitted := 0
	if maxShown > 0 && len(shown) > maxShown {
		shown = shown[:maxShown]
		omitted = len(entries) - maxShown
	}
	for _, entry := range shown {
		s += fmt.Sprintf("    %s -> %s\n", entry.OldPath, entry.NewPath)
	}
	if omitted > 0 {
		s += fmt.Sprintf("    ... and %d more (see full report)\n", omitted)
	}
	return s
}
