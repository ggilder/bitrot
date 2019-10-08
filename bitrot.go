package main

import (
	"fmt"
	"github.com/jessevdk/go-flags"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	name    = "bitrot"
	version = "0.0.2"
)

// go-flags requires us to wrap positional args in a struct
type PathArguments struct {
	Path flags.Filename `positional-arg-name:"PATH" description:"Path to directory."`
}

type ComparedPathArguments struct {
	Old flags.Filename `positional-arg-name:"OLDPATH" description:"Path to old or original directory."`
	New flags.Filename `positional-arg-name:"NEWPATH" description:"Path to new or copy directory."`
}

// Options/arguments for the `generate` command
type Generate struct {
	Exclude   []string      `short:"e" long:"exclude" description:"File/directory names to exclude. Repeat option to exclude multiple names."`
	Pretty    bool          `short:"p" long:"pretty" description:"Make a \"pretty\" (indented) JSON file."`
	Arguments PathArguments `required:"true" positional-args:"true"`
	logger    *log.Logger
}

// Options/arguments for the `validate` command
type Validate struct {
	Exclude   []string      `short:"e" long:"exclude" description:"File/directory names to exclude. Repeat option to exclude multiple names."`
	Arguments PathArguments `required:"true" positional-args:"true"`
	logger    *log.Logger
}

// Options/arguments for the `compare` command
type Compare struct {
	Exclude   []string              `short:"e" long:"exclude" description:"File/directory names to exclude. Repeat option to exclude multiple names."`
	Arguments ComparedPathArguments `required:"true" positional-args:"true"`
	logger    *log.Logger
}

// Options/arguments for the `compare-latest-manifests` command
type CompareLatestManifests struct {
	Exclude   []string              `short:"e" long:"exclude" description:"File/directory names to exclude. Repeat option to exclude multiple names."`
	Arguments ComparedPathArguments `required:"true" positional-args:"true"`
	logger    *log.Logger
}

// Extracts string path from wrapper and converts it to an absolute path
func pathString(name flags.Filename) (string, error) {
	path, err := filepath.Abs(string(name))
	if err != nil {
		return "", err
	}
	return path, nil
}

func (cmd *Generate) Execute(args []string) (err error) {
	config := DefaultConfig()
	if len(cmd.Exclude) > 0 {
		config.ExcludedFiles = cmd.Exclude
	}
	assertNoExtraArgs(&args, cmd.logger)
	path, err := pathString(cmd.Arguments.Path)
	if err != nil {
		return err
	}
	manifestStorage := config.ManifestStorage()

	cmd.logger.Printf("Generating manifest for %s...\n", path)

	manifest, err := NewManifest(path, config)
	if err != nil {
		return err
	}

	// Potentially validate manifest against previous
	latestManifest, err := manifestStorage.LatestManifestForPath(path)
	if err != nil {
		return err
	}
	if latestManifest != nil {
		ts := latestManifest.CreatedAt.Format(manifestNameTimeFormat)
		cmd.logger.Printf("Comparing to previous manifest from %s\n", ts)
		comparison := CompareManifests(latestManifest, manifest)
		cmd.logger.Printf(manifestComparisonReportString(comparison))
	}

	// Write new manifest
	err = manifestStorage.AddManifest(manifest)
	if err != nil {
		cmd.logger.Fatalf("Error saving manifest! %s\n", err)
		return err
	}

	cmd.logger.Printf("Wrote manifest in %s\n", manifestStorage.Path)

	return nil
}

func (cmd *Validate) Execute(args []string) (err error) {
	config := DefaultConfig()
	if len(cmd.Exclude) > 0 {
		config.ExcludedFiles = cmd.Exclude
	}
	assertNoExtraArgs(&args, cmd.logger)
	path, err := pathString(cmd.Arguments.Path)
	if err != nil {
		return err
	}
	manifestStorage := config.ManifestStorage()

	cmd.logger.Printf("Validating manifest for %s...\n", path)

	currentManifest, err := NewManifest(path, config)
	if err != nil {
		return err
	}

	latestManifest, err := manifestStorage.LatestManifestForPath(path)
	if err != nil {
		return err
	}

	if latestManifest == nil {
		cmd.logger.Printf("No previous manifest to validate for %s.", path)
		return fmt.Errorf("")
	}

	comparison := CompareManifests(latestManifest, currentManifest)
	cmd.logger.Printf(manifestComparisonReportString(comparison))

	flagged := len(comparison.FlaggedPaths)
	if flagged > 0 {
		cmd.logger.Printf("%d files flagged for possible corruption.", flagged)
		return fmt.Errorf("")
	} else {
		cmd.logger.Printf("Validated manifest for %s.\n", path)
	}

	return nil
}

func (cmd *Compare) Execute(args []string) (err error) {
	config := DefaultConfig()
	if len(cmd.Exclude) > 0 {
		config.ExcludedFiles = cmd.Exclude
	}
	assertNoExtraArgs(&args, cmd.logger)
	oldPath, err := pathString(cmd.Arguments.Old)
	if err != nil {
		return err
	}

	newPath, err := pathString(cmd.Arguments.New)
	if err != nil {
		return err
	}

	oldManifest, err := NewManifest(oldPath, config)
	if err != nil {
		return err
	}

	newManifest, err := NewManifest(newPath, config)
	if err != nil {
		return err
	}

	comparison := CompareManifests(oldManifest, newManifest)
	cmd.logger.Printf(manifestComparisonReportString(comparison))

	flagged := len(comparison.FlaggedPaths)
	if flagged > 0 {
		cmd.logger.Printf("%d files flagged for possible corruption.", flagged)
		return fmt.Errorf("")
	} else {
		cmd.logger.Printf("Successfully validated %s as a copy of %s.\n", newPath, oldPath)
	}

	return nil
}

func (cmd *CompareLatestManifests) Execute(args []string) (err error) {
	config := DefaultConfig()
	if len(cmd.Exclude) > 0 {
		config.ExcludedFiles = cmd.Exclude
	}
	assertNoExtraArgs(&args, cmd.logger)
	manifestStorage := config.ManifestStorage()

	oldPath, err := pathString(cmd.Arguments.Old)
	if err != nil {
		return err
	}

	newPath, err := pathString(cmd.Arguments.New)
	if err != nil {
		return err
	}

	oldManifest, err := manifestStorage.LatestManifestForPath(oldPath)
	if err != nil {
		return err
	}

	newManifest, err := manifestStorage.LatestManifestForPath(newPath)
	if err != nil {
		return err
	}

	if oldManifest == nil {
		cmd.logger.Printf("No existing manifest for %s\n", oldPath)
		return nil
	}
	if newManifest == nil {
		cmd.logger.Printf("No existing manifest for %s\n", newPath)
		return nil
	}

	comparison := CompareManifests(oldManifest, newManifest)
	cmd.logger.Printf(manifestComparisonReportString(comparison))

	flagged := len(comparison.FlaggedPaths)
	if flagged > 0 {
		cmd.logger.Printf("%d files flagged for possible corruption.", flagged)
		return fmt.Errorf("")
	} else {
		cmd.logger.Printf("Successfully validated %s as a copy of %s.\n", newPath, oldPath)
	}

	return nil
}

func manifestComparisonReportString(comparison *ManifestComparison) string {
	return pathSection("Added", comparison.AddedPaths) +
		pathSection("Deleted", comparison.DeletedPaths) +
		renameSection(comparison.RenamedPaths) +
		pathSection("Modified", comparison.ModifiedPaths) +
		pathSection("Flagged", comparison.FlaggedPaths)
}

func pathSection(description string, paths []string) string {
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

func renameSection(entries []RenamedPath) string {
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
		"generate",
		"Generate manifest",
		"Generate manifest for directory",
		&Generate{logger: logger},
	)
	addCommand(
		parser,
		"validate",
		"Validate manifest",
		"Validate manifest for directory",
		&Validate{logger: logger},
	)
	addCommand(
		parser,
		"compare",
		"Compare manifests",
		"Compare manifests for two directories",
		&Compare{logger: logger},
	)
	addCommand(
		parser,
		"compare-latest-manifests",
		"Compare latest manifests",
		"Compare latest manifests for two directories",
		&CompareLatestManifests{logger: logger},
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
