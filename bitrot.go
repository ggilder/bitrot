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

type ManifestFile struct {
	Manifest  *Manifest
	JSONBytes []byte
	Filename  string
}

// TODO break this func down a bit and unit test?
func NewManifestFile(manifest *Manifest, pretty bool) *ManifestFile {
	var jsonBytes []byte
	var err error
	if pretty {
		jsonBytes, err = json.MarshalIndent(manifest, "", "  ")
	} else {
		jsonBytes, err = json.Marshal(manifest)
	}
	check(err)
	manifestName := fmt.Sprintf(
		manifestNameTemplate,
		manifest.CreatedAt.Format(manifestNameTimeFormat),
		shortChecksum(&jsonBytes),
	)
	return &ManifestFile{
		Manifest:  manifest,
		JSONBytes: jsonBytes,
		Filename:  manifestName,
	}
}

func LatestManifestFileForPath(path string) *ManifestFile {
	manifestDir := filepath.Join(path, manifestDirName)
	manifestPaths, err := filepath.Glob(filepath.Join(manifestDir, manifestGlob))
	check(err)
	sort.Sort(sort.Reverse(sort.StringSlice(manifestPaths)))

	if len(manifestPaths) == 0 {
		return nil
	}

	manifestPath := manifestPaths[0]
	jsonBytes, err := ioutil.ReadFile(manifestPath)
	check(err)

	var manifest Manifest
	err = json.Unmarshal(jsonBytes, &manifest)
	check(err)

	return &ManifestFile{
		Manifest:  &manifest,
		JSONBytes: jsonBytes,
		Filename:  manifestPath,
	}
}

// Extracts string path from wrapper and converts it to an absolute path
func (args *PathArguments) PathString() string {
	path, err := filepath.Abs(string(args.Path))
	check(err)
	return path
}

func (cmd *Generate) Execute(args []string) (err error) {
	config := DefaultConfig()
	if len(cmd.Exclude) > 0 {
		config.ExcludedFiles = cmd.Exclude
	}
	assertNoExtraArgs(&args, cmd.logger)
	path := cmd.Arguments.PathString()

	cmd.logger.Printf("Generating manifest for %s...\n", path)

	// Prepare manifest file destination
	manifestDir := filepath.Join(path, manifestDirName)
	// Using MkdirAll because it doesn't return an error when the path is already a directory
	err = os.MkdirAll(manifestDir, 0755)
	check(err)

	manifest := NewManifest(path, config)
	manifestFile := NewManifestFile(manifest, cmd.Pretty)

	manifestPath := filepath.Join(manifestDir, manifestFile.Filename)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		err = ioutil.WriteFile(manifestPath, manifestFile.JSONBytes, 0644)
		check(err)

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
	path := cmd.Arguments.PathString()

	cmd.logger.Printf("Validating manifest for %s...\n", path)

	currentManifest := NewManifest(path, config)
	latestManifestFile := LatestManifestFileForPath(path)

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

// General helper functions
func check(e error) {
	if e != nil {
		panic(e)
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
	var parser = flags.NewParser(&AppOpts, flags.Default)
	generate := Generate{logger: logger}
	validate := Validate{logger: logger}
	var err error
	_, err = parser.AddCommand("generate", "Generate manifest", "Generate manifest for directory", &generate)
	check(err)
	_, err = parser.AddCommand("validate", "Validate manifest", "Validate manifest for directory", &validate)
	check(err)
	parser.Parse()
}
