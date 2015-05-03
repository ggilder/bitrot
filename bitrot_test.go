package main

import (
	"bytes"
	"fmt"
	"github.com/jessevdk/go-flags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

func TestPathStringExtractsPath(t *testing.T) {
	args := PathArguments{Path: "/foo/bar"}
	path := pathString(args.Path)
	assert.Equal(t, "/foo/bar", path)
}

func TestPathStringIsAbsolute(t *testing.T) {
	dir, _ := os.Getwd()
	args := PathArguments{Path: "."}
	path := pathString(args.Path)
	assert.Equal(t, dir, path)
}

type CommandsIntegrationTestSuite struct {
	suite.Suite
	tempDir   string
	logger    *log.Logger
	logBuffer bytes.Buffer
}

func (suite *CommandsIntegrationTestSuite) SetupTest() {
	var err error
	suite.tempDir, err = ioutil.TempDir("", "checksum")
	check(err)
	suite.clearLog()
	suite.logger = log.New(&suite.logBuffer, "", 0)
}

func (suite *CommandsIntegrationTestSuite) TearDownTest() {
	os.RemoveAll(suite.tempDir)
}

func (suite *CommandsIntegrationTestSuite) copyTempDir() string {
	tempDirCopy, err := ioutil.TempDir("", "checksum")
	check(err)

	// super dumb directory copy
	err = filepath.Walk(suite.tempDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if fi.Mode().IsRegular() {
			relPath, err := filepath.Rel(suite.tempDir, path)
			check(err)
			destPath := filepath.Join(tempDirCopy, relPath)
			dir := filepath.Dir(destPath)
			check(os.MkdirAll(dir, 0755))
			data, err := ioutil.ReadFile(path)
			check(err)
			check(ioutil.WriteFile(destPath, data, 0644))
			err = os.Chtimes(destPath, fi.ModTime(), fi.ModTime())
			if err != nil {
				return err
			}
		}
		return nil
	})
	return tempDirCopy
}

func (suite *CommandsIntegrationTestSuite) writeTestFile(path, content string) {
	testFile := filepath.Join(suite.tempDir, path)
	dir := filepath.Dir(testFile)
	check(os.MkdirAll(dir, 0755))
	err := ioutil.WriteFile(testFile, []byte(content), 0644)
	check(err)
}

func (suite *CommandsIntegrationTestSuite) corruptTestFile(path string) {
	testFile := filepath.Join(suite.tempDir, path)
	stat, err := os.Stat(testFile)
	check(err)
	contents, err := ioutil.ReadFile(testFile)
	check(err)
	contents[0] = contents[0] ^ 255
	ioutil.WriteFile(testFile, contents, 0644)

	check(suite.backdateTestFile(path, stat.ModTime()))
}

func (suite *CommandsIntegrationTestSuite) deleteTestFile(path string) {
	testFile := filepath.Join(suite.tempDir, path)
	check(os.Remove(testFile))
}

func (suite *CommandsIntegrationTestSuite) backdateTestFile(path string, to time.Time) error {
	testFile := filepath.Join(suite.tempDir, path)
	return os.Chtimes(testFile, to, to)
}

func (suite *CommandsIntegrationTestSuite) clearLog() {
	suite.logBuffer.Reset()
}

func (suite *CommandsIntegrationTestSuite) generateCommand() *Generate {
	return &Generate{
		Arguments: PathArguments{
			Path: flags.Filename(suite.tempDir),
		},
		logger: suite.logger,
	}
}

func (suite *CommandsIntegrationTestSuite) validateCommand() *Validate {
	return &Validate{
		Arguments: PathArguments{
			Path: flags.Filename(suite.tempDir),
		},
		logger: suite.logger,
	}
}

func (suite *CommandsIntegrationTestSuite) compareCommand(oldPath string) *Compare {
	return &Compare{
		Arguments: ComparedPathArguments{
			Old: flags.Filename(oldPath),
			New: flags.Filename(suite.tempDir),
		},
		logger: suite.logger,
	}
}

func (suite *CommandsIntegrationTestSuite) compareLatestManifestsCommand(oldPath string) *CompareLatestManifests {
	return &CompareLatestManifests{
		Arguments: ComparedPathArguments{
			Old: flags.Filename(oldPath),
			New: flags.Filename(suite.tempDir),
		},
		logger: suite.logger,
	}
}

func (suite *CommandsIntegrationTestSuite) LogContains(text string) {
	suite.Contains(suite.logBuffer.String(), text)
}

func (suite *CommandsIntegrationTestSuite) TestLatestManifestFileForPath() {
	suite.writeTestFile("foo/bar", helloWorldString)
	generate := suite.generateCommand()
	generate.Execute([]string{})
	manifestPaths, err := filepath.Glob(filepath.Join(suite.tempDir, manifestDirName, manifestGlob))
	check(err)
	manifestPath := manifestPaths[0]

	// fake an older manifest
	oldManifestName := fmt.Sprintf(
		manifestNameTemplate,
		time.Now().Add(-10*time.Second).Format(manifestNameTimeFormat),
		"zzzzzzz",
	)
	check(os.Rename(manifestPath, filepath.Join(filepath.Dir(manifestPath), oldManifestName)))

	// add another file for the new manifest
	suite.writeTestFile("baz", helloWorldString)
	generate.Execute([]string{})
	manifestFile := LatestManifestFileForPath(suite.tempDir)
	suite.Len(manifestFile.Manifest.Entries, 2)
}

func (suite *CommandsIntegrationTestSuite) TestLatestManifestFileForPathWithNoManifest() {
	manifestFile := LatestManifestFileForPath(suite.tempDir)
	suite.Nil(manifestFile)
}

func (suite *CommandsIntegrationTestSuite) TestGenerateCommand() {
	suite.writeTestFile("foo/bar", helloWorldString)
	suite.generateCommand().Execute([]string{})
	suite.LogContains(fmt.Sprintf("Wrote manifest to %s", suite.tempDir))
}

func (suite *CommandsIntegrationTestSuite) TestGenerateCommandWithExistingManifest() {
	suite.writeTestFile("foo/bar", helloWorldString)
	suite.generateCommand().Execute([]string{})
	suite.clearLog()

	// Get SHA & date from manifest
	manifestPaths, err := filepath.Glob(filepath.Join(suite.tempDir, manifestDirName, manifestGlob))
	check(err)
	manifestPath := manifestPaths[0]
	re := regexp.MustCompile("manifest-([^-]+)-([^.]+).json$")
	matches := re.FindAllStringSubmatch(manifestPath, -1)
	ts := matches[0][1]

	suite.generateCommand().Execute([]string{})
	suite.LogContains(fmt.Sprintf("Comparing to previous manifest from %s", ts))
	suite.LogContains("Added paths: none")
	suite.LogContains("Deleted paths: none")
	suite.LogContains("Modified paths: none")
	suite.LogContains("Flagged paths: none")
}

func (suite *CommandsIntegrationTestSuite) TestGenerateCommandWithExistingManifestFailure() {
	suite.writeTestFile("foo/flagged", helloWorldString)
	suite.writeTestFile("foo/modified", helloWorldString)
	check(suite.backdateTestFile("foo/modified", time.Now().Add(-1*time.Minute)))
	suite.writeTestFile("foo/deleted", helloWorldString)
	suite.generateCommand().Execute([]string{})
	suite.clearLog()

	suite.writeTestFile("foo/added", helloWorldString)
	suite.writeTestFile("foo/modified", "")
	suite.corruptTestFile("foo/flagged")
	suite.deleteTestFile("foo/deleted")

	suite.generateCommand().Execute([]string{})
	suite.LogContains("Added paths:\n    foo/added")
	suite.LogContains("Deleted paths:\n    foo/deleted")
	suite.LogContains("Modified paths:\n    foo/modified")
	suite.LogContains("Flagged paths:\n    foo/flagged")
}

func (suite *CommandsIntegrationTestSuite) TestValidateCommand() {
	suite.writeTestFile("foo/bar", helloWorldString)
	suite.generateCommand().Execute([]string{})
	suite.clearLog()
	suite.validateCommand().Execute([]string{})
	suite.LogContains(fmt.Sprintf("Validated manifest for %s.", suite.tempDir))
}

func (suite *CommandsIntegrationTestSuite) TestValidateCommandFailure() {
	suite.writeTestFile("foo/bar", helloWorldString)
	suite.generateCommand().Execute([]string{})
	suite.corruptTestFile("foo/bar")
	suite.clearLog()
	suite.validateCommand().Execute([]string{})
	suite.LogContains("Flagged paths:\n    foo/bar\n")
}

func (suite *CommandsIntegrationTestSuite) TestValidateWithNoManifests() {
	suite.writeTestFile("foo/bar", helloWorldString)
	suite.validateCommand().Execute([]string{})
	suite.LogContains(fmt.Sprintf("No previous manifest to validate for %s.", suite.tempDir))
}

func (suite *CommandsIntegrationTestSuite) TestCompare() {
	suite.writeTestFile("foo/bar", helloWorldString)
	oldTempDir := suite.copyTempDir()
	suite.compareCommand(oldTempDir).Execute([]string{})
	suite.LogContains(fmt.Sprintf("Successfully validated %s as a copy of %s.\n", suite.tempDir, oldTempDir))
}

func (suite *CommandsIntegrationTestSuite) TestCompareWithFailures() {
	suite.writeTestFile("foo/flagged", helloWorldString)
	suite.writeTestFile("foo/modified", helloWorldString)
	check(suite.backdateTestFile("foo/modified", time.Now().Add(-1*time.Minute)))
	suite.writeTestFile("foo/deleted", helloWorldString)
	oldTempDir := suite.copyTempDir()

	// make modifications
	suite.writeTestFile("foo/added", helloWorldString)
	suite.writeTestFile("foo/modified", "")
	suite.corruptTestFile("foo/flagged")
	suite.deleteTestFile("foo/deleted")

	suite.compareCommand(oldTempDir).Execute([]string{})
	suite.LogContains("1 files flagged for possible corruption.")
	suite.LogContains("Added paths:\n    foo/added")
	suite.LogContains("Deleted paths:\n    foo/deleted")
	suite.LogContains("Modified paths:\n    foo/modified")
	suite.LogContains("Flagged paths:\n    foo/flagged")
}

func (suite *CommandsIntegrationTestSuite) TestCompareLatestManifests() {
	suite.writeTestFile("foo/bar", helloWorldString)
	suite.generateCommand().Execute([]string{})
	oldTempDir := suite.copyTempDir()
	suite.generateCommand().Execute([]string{})
	suite.compareLatestManifestsCommand(oldTempDir).Execute([]string{})
	suite.LogContains(fmt.Sprintf("Successfully validated %s as a copy of %s.\n", suite.tempDir, oldTempDir))
}

func (suite *CommandsIntegrationTestSuite) TestCompareLatestManifestsMissingManifest() {
	oldTempDir := suite.copyTempDir()
	suite.generateCommand().Execute([]string{})
	suite.compareLatestManifestsCommand(oldTempDir).Execute([]string{})
	suite.LogContains(fmt.Sprintf("No existing manifest for %s\n", oldTempDir))

	err := os.Rename(filepath.Join(suite.tempDir, ".bitrot"), filepath.Join(oldTempDir, ".bitrot"))
	check(err)
	suite.compareLatestManifestsCommand(oldTempDir).Execute([]string{})
	suite.LogContains(fmt.Sprintf("No existing manifest for %s\n", suite.tempDir))
}

func TestCommandsIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(CommandsIntegrationTestSuite))
}
