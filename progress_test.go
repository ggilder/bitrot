package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatProgressLineWithoutEstimate(t *testing.T) {
	line := formatProgressLine(ProgressStats{
		Scanned:     100,
		BytesHashed: 10 * 1024 * 1024,
		Elapsed:     10 * time.Second,
	})

	assert.Contains(t, line, "Scanned 100 files")
	assert.Contains(t, line, "10.0MiB hashed")
	assert.Contains(t, line, "1.0 MiB/s")
	assert.Contains(t, line, "elapsed 10s")
	assert.Contains(t, line, "0 errors")
	assert.NotContains(t, line, "ETA")
	assert.NotContains(t, line, "remaining")
}

func TestFormatProgressLineWithEstimate(t *testing.T) {
	line := formatProgressLine(ProgressStats{
		Scanned:        100,
		EstimatedTotal: 1000,
		BytesHashed:    10 * 1024 * 1024,
		Errored:        2,
		Elapsed:        10 * time.Second,
	})

	assert.Contains(t, line, "Scanned 100/~1000 files (10.0%, ~900 remaining)")
	assert.Contains(t, line, "ETA 1m30s")
	assert.Contains(t, line, "2 errors")
}

func TestFormatProgressLineEstimateDoesNotGoNegativeWhenOverrun(t *testing.T) {
	// A scan can exceed the previous manifest's entry count (files were
	// added since the last run); remaining/pct shouldn't go negative.
	line := formatProgressLine(ProgressStats{
		Scanned:        1200,
		EstimatedTotal: 1000,
		Elapsed:        time.Second,
	})

	assert.Contains(t, line, "~0 remaining")
}

func TestHumanBytes(t *testing.T) {
	assert.Equal(t, "512B", humanBytes(512))
	assert.Equal(t, "1.0KiB", humanBytes(1024))
	assert.Equal(t, "1.5MiB", humanBytes(1024*1024+512*1024))
	assert.Equal(t, "2.0GiB", humanBytes(2*1024*1024*1024))
}

func TestProgressPrinterPadsOverShorterSubsequentLines(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressPrinter(&buf)

	p.Update(ProgressStats{Scanned: 100000, EstimatedTotal: 999999, Elapsed: time.Minute})
	firstLen := len(strings.TrimPrefix(buf.String(), "\r"))
	buf.Reset()

	p.Update(ProgressStats{Scanned: 1, Elapsed: time.Second})
	second := strings.TrimPrefix(buf.String(), "\r")

	assert.True(t, strings.HasPrefix(second, "Scanned 1 files"))
	// Padded out to at least as wide as the longest line seen so far, so no
	// leftover characters from the previous (longer) line remain visible.
	assert.GreaterOrEqual(t, len(second), firstLen)
}

func TestProgressPrinterFinishEndsWithNewline(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressPrinter(&buf)
	p.Update(ProgressStats{Scanned: 1})
	p.Finish()

	assert.True(t, strings.HasSuffix(buf.String(), "\n"))
}
