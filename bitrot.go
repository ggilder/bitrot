package main

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jessevdk/go-flags"
	"hash/crc32"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	name                 = "bitrot"
	version              = "0.0.1"
	manifestDirName      = ".bitrot"
	manifestNameTemplate = "manifest-%s-%s.json"
	manifestGlob         = "manifest-*.json"
	// RFC3339 minus punctuation characters, better for filenames
	manifestNameTimeFormat = "20060102T150405Z07:00"
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
	Arguments ComparedPathArguments `required:"true" positional-args:"true"`
	logger    *log.Logger
}

type ManifestFile struct {
	Manifest  *Manifest
	JSONBytes []byte
	Filename  string
}

// TODO break this func down a bit and unit test?
func NewManifestFile(manifest *Manifest, pretty bool) (*ManifestFile, error) {
	var jsonBytes []byte
	var err error
	if pretty {
		jsonBytes, err = json.MarshalIndent(manifest, "", "  ")
	} else {
		jsonBytes, err = json.Marshal(manifest)
	}
	if err != nil {
		return nil, err
	}
	manifestName := fmt.Sprintf(
		manifestNameTemplate,
		manifest.CreatedAt.Format(manifestNameTimeFormat),
		shortChecksum(&jsonBytes),
	)
	return &ManifestFile{
		Manifest:  manifest,
		JSONBytes: jsonBytes,
		Filename:  manifestName,
	}, nil
}

func LatestManifestFileForPath(path string) (*ManifestFile, error) {
	manifestDir := filepath.Join(path, manifestDirName)
	manifestPaths, err := filepath.Glob(filepath.Join(manifestDir, manifestGlob))
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(manifestPaths)))

	if len(manifestPaths) == 0 {
		return nil, nil
	}

	manifestPath := manifestPaths[0]
	jsonBytes, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	var manifest Manifest
	err = json.Unmarshal(jsonBytes, &manifest)
	if err != nil {
		return nil, err
	}

	return &ManifestFile{
		Manifest:  &manifest,
		JSONBytes: jsonBytes,
		Filename:  manifestPath,
	}, nil
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

	cmd.logger.Printf("Generating manifest for %s...\n", path)

	// Prepare manifest file destination
	manifestDir := filepath.Join(path, manifestDirName)
	// Using MkdirAll because it doesn't return an error when the path is already a directory
	err = os.MkdirAll(manifestDir, 0755)
	if err != nil {
		return
	}

	manifest, err := NewManifest(path, config)
	if err != nil {
		return err
	}

	// Potentially validate manifest against previous
	latestManifestFile, err := LatestManifestFileForPath(path)
	if err != nil {
		return err
	}
	if latestManifestFile != nil {
		ts := latestManifestFile.Manifest.CreatedAt.Format(manifestNameTimeFormat)
		cmd.logger.Printf("Comparing to previous manifest from %s\n", ts)
		comparison := CompareManifests(latestManifestFile.Manifest, manifest)
		cmd.logger.Printf(manifestComparisonReportString(comparison))
	}

	// Write new manifest
	manifestFile, err := NewManifestFile(manifest, cmd.Pretty)
	if err != nil {
		return err
	}

	manifestPath := filepath.Join(manifestDir, manifestFile.Filename)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		err = ioutil.WriteFile(manifestPath, manifestFile.JSONBytes, 0644)
		if err != nil {
			return err
		}

		cmd.logger.Printf("Wrote manifest to %s\n", manifestPath)
	} else {
		cmd.logger.Fatalf("Manifest file already exists! Path: %s\n", manifestPath)
	}

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

	cmd.logger.Printf("Validating manifest for %s...\n", path)

	currentManifest, err := NewManifest(path, config)
	if err != nil {
		return err
	}

	latestManifestFile, err := LatestManifestFileForPath(path)
	if err != nil {
		return err
	}

	if latestManifestFile == nil {
		// TODO: want to use Fatalf here but can't seem to catch it in tests
		cmd.logger.Printf("No previous manifest to validate for %s.\n", path)
		return nil
	}

	comparison := CompareManifests(latestManifestFile.Manifest, currentManifest)
	cmd.logger.Printf(manifestComparisonReportString(comparison))

	flagged := len(comparison.FlaggedPaths)
	if flagged > 0 {
		// TODO: want to use Fatalf here but can't seem to catch it in tests
		cmd.logger.Printf("%d files flagged for possible corruption.\n", flagged)
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
		// TODO: want to use Fatalf here but can't seem to catch it in tests
		cmd.logger.Printf("%d files flagged for possible corruption.\n", flagged)
	} else {
		cmd.logger.Printf("Successfully validated %s as a copy of %s.\n", newPath, oldPath)
	}

	return nil
}

func (cmd *CompareLatestManifests) Execute(args []string) (err error) {
	assertNoExtraArgs(&args, cmd.logger)
	oldPath, err := pathString(cmd.Arguments.Old)
	if err != nil {
		return err
	}

	newPath, err := pathString(cmd.Arguments.New)
	if err != nil {
		return err
	}

	oldManifest, err := LatestManifestFileForPath(oldPath)
	if err != nil {
		return err
	}

	newManifest, err := LatestManifestFileForPath(newPath)
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

	comparison := CompareManifests(oldManifest.Manifest, newManifest.Manifest)
	cmd.logger.Printf(manifestComparisonReportString(comparison))

	flagged := len(comparison.FlaggedPaths)
	if flagged > 0 {
		// TODO: want to use Fatalf here but can't seem to catch it in tests
		cmd.logger.Printf("%d files flagged for possible corruption.\n", flagged)
	} else {
		cmd.logger.Printf("Successfully validated %s as a copy of %s.\n", newPath, oldPath)
	}

	return nil
}

func manifestComparisonReportString(comparison *ManifestComparison) string {
	return pathSection("Added", comparison.AddedPaths) +
		pathSection("Deleted", comparison.DeletedPaths) +
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

func assertNoExtraArgs(args *[]string, logger *log.Logger) {
	if len(*args) > 0 {
		logger.Fatalf("Unrecognized arguments: %s\n", strings.Join(*args, " "))
	}
}

// Short checksum suitable for a quick check on the manifest files
func shortChecksum(data *[]byte) string {
	checksum := crc32.ChecksumIEEE(*data)
	checksumBytes := [4]byte{}
	binary.BigEndian.PutUint32(checksumBytes[:], checksum)
	return hex.EncodeToString(checksumBytes[:])
}

func addCommand(parser *flags.Parser, name, summary, description string, command interface{}) {
	_, err := parser.AddCommand(name, summary, description, command)
	if err != nil {
		panic(err)
	}
}

func main() {
	logger := log.New(os.Stderr, "", 0)
	var AppOpts struct {
		Version func() `long:"version" short:"v"`
	}
	AppOpts.Version = func() {
		log.Printf("%s version %s\n", name, version)
		os.Exit(0)
	}
	parser := flags.NewParser(&AppOpts, flags.Default)
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
	parser.Parse()
}
