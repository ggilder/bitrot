package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/go-homedir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// General helper functions

func TestPathStringExtractsPath(t *testing.T) {
	args := PathArguments{Path: "/foo/bar"}
	path, err := pathString(args.Path)
	assert.Nil(t, err)
	assert.Equal(t, "/foo/bar", path)
}

func TestPathStringIsAbsolute(t *testing.T) {
	dir, _ := os.Getwd()
	args := PathArguments{Path: "."}
	path, err := pathString(args.Path)
	assert.Nil(t, err)
	assert.Equal(t, dir, path)
}

type CommandsIntegrationTestSuite struct {
	suite.Suite
	tempDir   string
	homeDir   string
	logger    *log.Logger
	logBuffer bytes.Buffer
}

func (suite *CommandsIntegrationTestSuite) SetupTest() {
	var err error
	homedir.DisableCache = true
	suite.tempDir, err = ioutil.TempDir("", "checksum")
	assert.Nil(suite.T(), err)
	suite.homeDir, err = ioutil.TempDir("", "home")
	assert.Nil(suite.T(), err)
	suite.clearLog()
	suite.logger = log.New(&suite.logBuffer, "", 0)
	os.Setenv("HOME", suite.homeDir)
}

func (suite *CommandsIntegrationTestSuite) TearDownTest() {
	os.RemoveAll(suite.tempDir)
	os.RemoveAll(suite.homeDir)
}

func (suite *CommandsIntegrationTestSuite) copyTempDir() string {
	tempDirCopy, err := ioutil.TempDir("", "checksum")
	assert.Nil(suite.T(), err)

	// super dumb directory copy
	err = filepath.Walk(suite.tempDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if fi.Mode().IsRegular() {
			relPath, err := filepath.Rel(suite.tempDir, path)
			assert.Nil(suite.T(), err)
			destPath := filepath.Join(tempDirCopy, relPath)
			dir := filepath.Dir(destPath)
			assert.Nil(suite.T(), os.MkdirAll(dir, 0755))
			data, err := ioutil.ReadFile(path)
			assert.Nil(suite.T(), err)
			assert.Nil(suite.T(), ioutil.WriteFile(destPath, data, 0644))
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
	assert.Nil(suite.T(), os.MkdirAll(dir, 0755))
	err := ioutil.WriteFile(testFile, []byte(content), 0644)
	assert.Nil(suite.T(), err)
}

func (suite *CommandsIntegrationTestSuite) corruptTestFile(path string) {
	testFile := filepath.Join(suite.tempDir, path)
	stat, err := os.Stat(testFile)
	assert.Nil(suite.T(), err)
	contents, err := ioutil.ReadFile(testFile)
	assert.Nil(suite.T(), err)
	contents[0] = contents[0] ^ 255
	ioutil.WriteFile(testFile, contents, 0644)

	assert.Nil(suite.T(), suite.backdateTestFile(path, stat.ModTime()))
}

func (suite *CommandsIntegrationTestSuite) deleteTestFile(path string) {
	testFile := filepath.Join(suite.tempDir, path)
	assert.Nil(suite.T(), os.Remove(testFile))
}

func (suite *CommandsIntegrationTestSuite) backdateTestFile(path string, to time.Time) error {
	testFile := filepath.Join(suite.tempDir, path)
	return os.Chtimes(testFile, to, to)
}

func (suite *CommandsIntegrationTestSuite) clearLog() {
	suite.logBuffer.Reset()
}

func (suite *CommandsIntegrationTestSuite) generateCommand(dir string) *Generate {
	return &Generate{
		Arguments: PathArguments{
			Path: flags.Filename(dir),
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

func (suite *CommandsIntegrationTestSuite) TestGenerateCommand() {
	suite.writeTestFile("foo/bar", helloWorldString)
	err := suite.generateCommand(suite.tempDir).Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.LogContains(fmt.Sprintf("Wrote manifest"))
}

func (suite *CommandsIntegrationTestSuite) TestGenerateCommandWithExistingManifest() {
	suite.writeTestFile("foo/bar", helloWorldString)
	err := suite.generateCommand(suite.tempDir).Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.clearLog()

	// Get SHA & date from manifest
	manifestPaths, err := filepath.Glob(filepath.Join(suite.homeDir, configDir, configStorageDir, "*", manifestGlob))
	assert.Nil(suite.T(), err)
	manifestPath := manifestPaths[0]
	re := regexp.MustCompile("manifest-([^-]+)-([^.]+).json$")
	matches := re.FindAllStringSubmatch(manifestPath, -1)
	ts := matches[0][1]

	err = suite.generateCommand(suite.tempDir).Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.LogContains(fmt.Sprintf("Comparing to previous manifest from %s", ts))
	suite.LogContains("Added paths: none")
	suite.LogContains("Deleted paths: none")
	suite.LogContains("Modified paths: none")
	suite.LogContains("Flagged paths: none")
}

func (suite *CommandsIntegrationTestSuite) TestGenerateCommandWithExistingManifestFailure() {
	suite.writeTestFile("foo/flagged", helloWorldString)
	suite.writeTestFile("foo/modified", "to modify")
	assert.Nil(suite.T(), suite.backdateTestFile("foo/modified", time.Now().Add(-1*time.Minute)))
	suite.writeTestFile("foo/deleted", helloWorldString)
	err := suite.generateCommand(suite.tempDir).Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.clearLog()

	suite.writeTestFile("foo/added", "added")
	suite.writeTestFile("foo/modified", "modified")
	suite.corruptTestFile("foo/flagged")
	suite.deleteTestFile("foo/deleted")

	err = suite.generateCommand(suite.tempDir).Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.LogContains("Added paths:\n    foo/added")
	suite.LogContains("Deleted paths:\n    foo/deleted")
	suite.LogContains("Modified paths:\n    foo/modified")
	suite.LogContains("Flagged paths:\n    foo/flagged")
}

func (suite *CommandsIntegrationTestSuite) TestValidateCommand() {
	suite.writeTestFile("foo/bar", helloWorldString)

	err := suite.generateCommand(suite.tempDir).Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.clearLog()
	err = suite.validateCommand().Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.LogContains(fmt.Sprintf("Validated manifest for %s.", suite.tempDir))
}

func (suite *CommandsIntegrationTestSuite) TestValidateCommandFailure() {
	suite.writeTestFile("foo/bar", helloWorldString)
	err := suite.generateCommand(suite.tempDir).Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.corruptTestFile("foo/bar")
	suite.clearLog()
	err = suite.validateCommand().Execute([]string{})
	assert.NotNil(suite.T(), err)

	suite.LogContains("Flagged paths:\n    foo/bar\n")
}

func (suite *CommandsIntegrationTestSuite) TestValidateWithNoManifests() {
	suite.writeTestFile("foo/bar", helloWorldString)
	err := suite.validateCommand().Execute([]string{})
	assert.NotNil(suite.T(), err)

	suite.LogContains(fmt.Sprintf("No previous manifest to validate for %s.", suite.tempDir))
}

func (suite *CommandsIntegrationTestSuite) TestCompare() {
	suite.writeTestFile("foo/bar", helloWorldString)
	oldTempDir := suite.copyTempDir()
	err := suite.compareCommand(oldTempDir).Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.LogContains("Unchanged paths: 1\n")
	suite.LogContains(fmt.Sprintf("Successfully validated %s as a copy of %s.\n", suite.tempDir, oldTempDir))
}

func (suite *CommandsIntegrationTestSuite) TestCompareWithFailures() {
	suite.writeTestFile("foo/flagged", helloWorldString)
	suite.writeTestFile("foo/modified", "to modify")
	assert.Nil(suite.T(), suite.backdateTestFile("foo/modified", time.Now().Add(-1*time.Minute)))
	suite.writeTestFile("foo/deleted", helloWorldString)
	oldTempDir := suite.copyTempDir()

	// make modifications
	suite.writeTestFile("foo/added", "added")
	suite.writeTestFile("foo/modified", "modified")
	suite.corruptTestFile("foo/flagged")
	suite.deleteTestFile("foo/deleted")

	err := suite.compareCommand(oldTempDir).Execute([]string{})
	assert.NotNil(suite.T(), err)

	suite.LogContains("1 files flagged for possible corruption.")
	suite.LogContains("Unchanged paths: 0\n")
	suite.LogContains("Added paths:\n    foo/added")
	suite.LogContains("Deleted paths:\n    foo/deleted")
	suite.LogContains("Modified paths:\n    foo/modified")
	suite.LogContains("Flagged paths:\n    foo/flagged")
}

func (suite *CommandsIntegrationTestSuite) TestCompareWithRenames() {
	timestamp := time.Now()
	suite.writeTestFile("foo/testfile", helloWorldString)
	assert.Nil(suite.T(), suite.backdateTestFile("foo/testfile", timestamp))
	suite.writeTestFile("foo/deleted", "deleted")
	oldTempDir := suite.copyTempDir()

	// make modifications
	suite.writeTestFile("foo/added", "added")
	suite.writeTestFile("foo/testfile2", helloWorldString)
	assert.Nil(suite.T(), suite.backdateTestFile("foo/testfile2", timestamp))
	suite.deleteTestFile("foo/deleted")
	suite.deleteTestFile("foo/testfile")

	err := suite.compareCommand(oldTempDir).Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.LogContains(fmt.Sprintf("Successfully validated %s as a copy of %s.\n", suite.tempDir, oldTempDir))
	suite.LogContains("Unchanged paths: 0\n")
	suite.LogContains("Added paths:\n    foo/added")
	suite.LogContains("Deleted paths:\n    foo/deleted")
	suite.LogContains("Renamed paths:\n    foo/testfile -> foo/testfile2")
}

func (suite *CommandsIntegrationTestSuite) TestCompareLatestManifests() {
	suite.writeTestFile("foo/bar", helloWorldString)
	err := suite.generateCommand(suite.tempDir).Execute([]string{})
	assert.Nil(suite.T(), err)

	oldTempDir := suite.copyTempDir()
	err = suite.generateCommand(oldTempDir).Execute([]string{})
	assert.Nil(suite.T(), err)

	err = suite.compareLatestManifestsCommand(oldTempDir).Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.LogContains(fmt.Sprintf("Successfully validated %s as a copy of %s.\n", suite.tempDir, oldTempDir))
	suite.LogContains("Unchanged paths: 1\n")
}

func (suite *CommandsIntegrationTestSuite) TestCompareLatestManifestsMissingManifest() {
	oldTempDir := suite.copyTempDir()
	err := suite.generateCommand(suite.tempDir).Execute([]string{})
	assert.Nil(suite.T(), err)

	err = suite.compareLatestManifestsCommand(oldTempDir).Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.LogContains(fmt.Sprintf("No existing manifest for %s\n", oldTempDir))
}

func TestCommandsIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(CommandsIntegrationTestSuite))
}
