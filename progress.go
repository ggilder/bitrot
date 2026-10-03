package main

import (
	"fmt"
	"io"
	"time"
)

// ProgressStats is a snapshot of scan progress at a point in time.
type ProgressStats struct {
	Scanned int
	// EstimatedTotal is how many files the walk has discovered so far - it
	// grows while the walk is still running and becomes an exact total once
	// the walk finishes. 0 means nothing's been discovered yet.
	EstimatedTotal int
	// TotalIsExact is true once the walk itself has finished, at which
	// point EstimatedTotal stops growing and is the real total rather than
	// a lower bound still catching up.
	TotalIsExact bool
	BytesHashed  int64
	Errored      int
	Elapsed      time.Duration
}

// ProgressFunc is called periodically as files are processed. Implementations
// should be cheap; it's called from the same goroutine that aggregates scan
// results, so it can't block scanning for long without slowing the scan.
type ProgressFunc func(ProgressStats)

// ProgressPrinter renders ProgressStats as a single status line, rewritten
// in place (using \r) on w.
type ProgressPrinter struct {
	w        io.Writer
	maxWidth int
}

func NewProgressPrinter(w io.Writer) *ProgressPrinter {
	return &ProgressPrinter{w: w}
}

func (p *ProgressPrinter) Update(stats ProgressStats) {
	line := formatProgressLine(stats)
	if len(line) > p.maxWidth {
		p.maxWidth = len(line)
	}
	pad := p.maxWidth - len(line)
	fmt.Fprint(p.w, "\r"+line+spaces(pad))
}

// Finish ends the progress line so subsequent output starts on a fresh line.
func (p *ProgressPrinter) Finish() {
	fmt.Fprintln(p.w)
}

func formatProgressLine(stats ProgressStats) string {
	line := fmt.Sprintf("Scanned %d", stats.Scanned)

	var eta time.Duration
	haveETA := false
	if stats.EstimatedTotal > 0 {
		remaining := stats.EstimatedTotal - stats.Scanned
		if remaining < 0 {
			remaining = 0
		}
		pct := float64(stats.Scanned) / float64(stats.EstimatedTotal) * 100
		tilde := "~"
		if stats.TotalIsExact {
			tilde = ""
		}
		line += fmt.Sprintf("/%s%d files (%.1f%%, %s%d remaining)", tilde, stats.EstimatedTotal, pct, tilde, remaining)

		// ETA from the observed files/sec rate - only meaningful once we've
		// actually processed something and have files left to go.
		if stats.Scanned > 0 && remaining > 0 && stats.Elapsed.Seconds() > 0 {
			filesPerSec := float64(stats.Scanned) / stats.Elapsed.Seconds()
			eta = time.Duration(float64(remaining)/filesPerSec) * time.Second
			haveETA = true
		}
	} else {
		line += " files"
	}

	mbps := 0.0
	if stats.Elapsed.Seconds() > 0 {
		mbps = float64(stats.BytesHashed) / stats.Elapsed.Seconds() / (1024 * 1024)
	}
	line += fmt.Sprintf(" | %s hashed | %.1f MiB/s | elapsed %s",
		humanBytes(stats.BytesHashed), mbps, stats.Elapsed.Round(time.Second))
	if haveETA {
		line += fmt.Sprintf(" | ETA %s", eta.Round(time.Second))
	}
	line += fmt.Sprintf(" | %d errors", stats.Errored)

	return line
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
