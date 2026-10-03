package main

import (
	"fmt"
	"github.com/jessevdk/go-flags"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	name    = "bitrot"
	version = "0.1.0"
)

// go-flags requires us to wrap positional args in a struct
type PathArguments struct {
	Path flags.Filename `positional-arg-name:"PATH" description:"Path to directory."`
}

// Options/arguments for the `scan` command
type Scan struct {
	Exclude       []string      `short:"e" long:"exclude" description:"File/directory basename to exclude wherever it occurs in the tree, in addition to the built-in defaults (.DS_Store, .git, etc). Repeat option to exclude multiple names."`
	ExcludePrefix []string      `short:"x" long:"exclude-prefix" description:"Path, relative to PATH, to exclude from the scan along with everything under it. Repeat option to exclude multiple paths."`
	Workers       int           `short:"w" long:"workers" description:"Number of parallel hashing workers (defaults to number of CPUs)" default:"0"`
	LogFile       string        `short:"l" long:"log-file" description:"Write the full, untruncated report to this file."`
	Truncate      int           `short:"t" long:"truncate" description:"Maximum number of paths to print per section (0 = no truncation). Useful when piping the report somewhere size-constrained, like an email body." default:"0"`
	Progress      bool          `short:"p" long:"progress" description:"Print a live progress status line to stderr while scanning. Off by default since it's noise if captured into a log/email."`
	Arguments     PathArguments `required:"true" positional-args:"true"`
	logger        *log.Logger
}

// Extracts string path from wrapper and converts it to an absolute path
func pathString(name flags.Filename) (string, error) {
	path, err := filepath.Abs(string(name))
	if err != nil {
		return "", err
	}
	return path, nil
}

func (cmd *Scan) Execute(args []string) (err error) {
	config := DefaultConfig()
	if len(cmd.Exclude) > 0 {
		// Additive to the built-in defaults, so passing --exclude doesn't
		// silently stop ignoring .DS_Store etc.
		config.ExcludedNames = append(config.ExcludedNames, cmd.Exclude...)
	}
	if len(cmd.ExcludePrefix) > 0 {
		config.ExcludedPrefixes = cmd.ExcludePrefix
	}
	assertNoExtraArgs(&args, cmd.logger)
	path, err := pathString(cmd.Arguments.Path)
	if err != nil {
		return err
	}
	manifestStorage := config.ManifestStorage()

	// Fetch the previous manifest first (cheap - one file) so its entry
	// count can seed the progress estimate below.
	latestManifest, err := manifestStorage.LatestManifestForPath(path)
	if err != nil {
		return err
	}
	estimatedTotal := 0
	if latestManifest != nil {
		estimatedTotal = len(latestManifest.Entries)
	}

	cmd.logger.Printf("Scanning %s...\n", path)

	var progressFn ProgressFunc
	var progressPrinter *ProgressPrinter
	if cmd.Progress {
		progressPrinter = NewProgressPrinter(os.Stderr)
		progressFn = progressPrinter.Update
	}

	manifest, errored, err := NewManifest(path, config, cmd.Workers, estimatedTotal, progressFn)
	if progressPrinter != nil {
		progressPrinter.Finish()
	}
	if err != nil {
		return err
	}
	for _, fe := range errored {
		cmd.logger.Printf("Error reading %s: %s\n", fe.Path, fe.Error)
	}
	if len(errored) > 0 {
		cmd.logger.Printf("%d files could not be read.\n", len(errored))
	}

	var comparison *ManifestComparison
	if latestManifest != nil {
		ts := latestManifest.CreatedAt.Format(manifestNameTimeFormat)
		cmd.logger.Printf("Comparing to previous manifest from %s\n", ts)
		comparison = CompareManifests(latestManifest, manifest)
	}

	// Write new manifest
	err = manifestStorage.AddManifest(manifest)
	if err != nil {
		cmd.logger.Fatalf("Error saving manifest! %s\n", err)
		return err
	}
	cmd.logger.Printf("Wrote manifest in %s\n", manifestStorage.Path)

	if comparison == nil {
		cmd.logger.Printf("No previous manifest to compare for %s.\n", path)
		if len(errored) > 0 {
			return fmt.Errorf("")
		}
		return nil
	}

	report := NewComparisonReport(comparison)

	if cmd.LogFile != "" {
		if writeErr := ioutil.WriteFile(cmd.LogFile, []byte(report.ReportString()), 0644); writeErr != nil {
			cmd.logger.Printf("Error writing full report to %s: %s\n", cmd.LogFile, writeErr)
		} else {
			cmd.logger.Printf("Wrote full report to %s\n", cmd.LogFile)
		}
	}

	cmd.logger.Printf(report.TruncatedReportString(cmd.Truncate))

	deleted := len(comparison.DeletedPaths)
	flagged := len(comparison.FlaggedPaths)
	if deleted > 0 || flagged > 0 || len(errored) > 0 {
		cmd.logger.Printf("%d files deleted, %d files flagged for possible corruption, %d files could not be read.\n", deleted, flagged, len(errored))
		return fmt.Errorf("")
	}

	cmd.logger.Printf("Scan validated for %s.\n", path)
	return nil
}

func assertNoExtraArgs(args *[]string, logger *log.Logger) {
	if len(*args) > 0 {
		logger.Fatalf("Unrecognized arguments: %s\n", strings.Join(*args, " "))
	}
}

func addCommand(parser *flags.Parser, name, summary, description string, command interface{}) {
	_, err := parser.AddCommand(name, summary, description, command)
	if err != nil {
		panic(err)
	}
}

func main() {
	logger := log.New(os.Stdout, "", 0)
	var AppOpts struct {
		Version func() `long:"version" short:"v"`
	}
	AppOpts.Version = func() {
		logger.Printf("%s version %s\n", name, version)
		os.Exit(0)
	}
	parser := flags.NewParser(&AppOpts, flags.HelpFlag|flags.PassDoubleDash)
	addCommand(
		parser,
		"scan",
		"Scan directory",
		"Generate a manifest for a directory, compare it to the previous scan, and report additions/deletions/renames/modifications/flagged (possible corruption) files.",
		&Scan{logger: logger},
	)
	_, err := parser.Parse()
	if err != nil {
		// Ignore the "signal" errors produced by commands (which print their own error messages)
		if err.Error() != "" {
			logger.Println(err)
		}
		os.Exit(1)
	}
}
