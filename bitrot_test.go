package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
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

func (suite *CommandsIntegrationTestSuite) scanCommand(opts ...func(*Scan)) *Scan {
	cmd := &Scan{
		Arguments: PathArguments{
			Path: flags.Filename(suite.tempDir),
		},
		logger: suite.logger,
	}
	for _, opt := range opts {
		opt(cmd)
	}
	return cmd
}

func (suite *CommandsIntegrationTestSuite) LogContains(text string) {
	suite.Contains(suite.logBuffer.String(), text)
}

func (suite *CommandsIntegrationTestSuite) TestScanCommand() {
	suite.writeTestFile("foo/bar", helloWorldString)
	cmd := suite.scanCommand()
	err := cmd.Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.LogContains("No previous manifest to compare")

	// The log should name the exact manifest file written, not just the
	// storage root.
	assert.NotEmpty(suite.T(), cmd.WrittenManifestPath)
	suite.LogContains(fmt.Sprintf("Wrote manifest to %s", cmd.WrittenManifestPath))
	_, statErr := os.Stat(cmd.WrittenManifestPath)
	assert.Nil(suite.T(), statErr)
}

func (suite *CommandsIntegrationTestSuite) TestScanCommandWritesLogFileOnFirstScan() {
	suite.writeTestFile("foo/bar", helloWorldString)

	logFile := filepath.Join(suite.tempDir, "first-scan-report.txt")
	err := suite.scanCommand(func(cmd *Scan) {
		cmd.LogFile = logFile
	}).Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.LogContains("No previous manifest to compare")
	suite.LogContains(fmt.Sprintf("Wrote full report to %s", logFile))

	content, readErr := ioutil.ReadFile(logFile)
	assert.Nil(suite.T(), readErr)
	assert.Contains(suite.T(), string(content), "No previous manifest to compare")
}

func (suite *CommandsIntegrationTestSuite) TestScanCommandWithExistingManifestSuccess() {
	suite.writeTestFile("foo/bar", helloWorldString)
	firstScan := suite.scanCommand()
	err := firstScan.Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.clearLog()

	ts, err := createdAtFromManifestFilename(firstScan.WrittenManifestPath)
	assert.Nil(suite.T(), err)

	err = suite.scanCommand().Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.LogContains(fmt.Sprintf("Comparing to previous manifest from %s", ts.Format(manifestNameTimeFormat)))
	suite.LogContains("Added paths: 0")
	suite.LogContains("Deleted paths: 0")
	suite.LogContains("Modified paths: 0")
	suite.LogContains("Flagged paths: 0")
	suite.LogContains("Scan validated for")
}

func (suite *CommandsIntegrationTestSuite) TestScanCommandWithExistingManifestFailure() {
	suite.writeTestFile("foo/flagged", helloWorldString)
	suite.writeTestFile("foo/modified", "to modify")
	assert.Nil(suite.T(), suite.backdateTestFile("foo/modified", time.Now().Add(-1*time.Minute)))
	suite.writeTestFile("foo/deleted", helloWorldString)
	err := suite.scanCommand().Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.clearLog()

	suite.writeTestFile("foo/added", "added")
	suite.writeTestFile("foo/modified", "modified")
	suite.corruptTestFile("foo/flagged")
	suite.deleteTestFile("foo/deleted")

	err = suite.scanCommand().Execute([]string{})
	assert.NotNil(suite.T(), err)

	suite.LogContains("Added paths: 1\n    foo/added")
	suite.LogContains("Deleted paths: 1\n    foo/deleted")
	suite.LogContains("Modified paths: 1\n    foo/modified")
	suite.LogContains("Flagged paths: 1\n    foo/flagged")
	suite.LogContains("1 files deleted, 1 files flagged for possible corruption, 0 files could not be read.")
}

func (suite *CommandsIntegrationTestSuite) TestScanCommandWithRenames() {
	timestamp := time.Now()
	suite.writeTestFile("foo/testfile", helloWorldString)
	assert.Nil(suite.T(), suite.backdateTestFile("foo/testfile", timestamp))
	suite.writeTestFile("foo/deleted", "deleted")
	err := suite.scanCommand().Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.clearLog()

	suite.writeTestFile("foo/added", "added")
	suite.writeTestFile("foo/testfile2", helloWorldString)
	assert.Nil(suite.T(), suite.backdateTestFile("foo/testfile2", timestamp))
	suite.deleteTestFile("foo/deleted")
	suite.deleteTestFile("foo/testfile")

	err = suite.scanCommand().Execute([]string{})
	assert.NotNil(suite.T(), err)

	suite.LogContains("Added paths: 1\n    foo/added")
	suite.LogContains("Deleted paths: 1\n    foo/deleted")
	suite.LogContains("Renamed paths: 1\n    foo/testfile -> foo/testfile2")
	suite.LogContains("1 files deleted, 0 files flagged for possible corruption, 0 files could not be read.")
}

func (suite *CommandsIntegrationTestSuite) TestScanCommandWithPrefixExclusions() {
	suite.writeTestFile("keep/me", helloWorldString)
	suite.writeTestFile("snapshots/weekly/2026-01-01/anything", helloWorldString)
	err := suite.scanCommand(func(cmd *Scan) {
		cmd.ExcludePrefix = []string{"snapshots/weekly"}
	}).Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.clearLog()

	// Pruning the excluded snapshot directory shouldn't register as a deletion
	assert.Nil(suite.T(), os.RemoveAll(filepath.Join(suite.tempDir, "snapshots")))

	err = suite.scanCommand(func(cmd *Scan) {
		cmd.ExcludePrefix = []string{"snapshots/weekly"}
	}).Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.LogContains("Deleted paths: 0\n")
	suite.LogContains("Scan validated for")
}

func (suite *CommandsIntegrationTestSuite) TestScanCommandSkipsUnreadableExcludedDirectory() {
	suite.writeTestFile("keep/me", helloWorldString)

	unreadableDir := filepath.Join(suite.tempDir, "unreadable-excluded")
	assert.Nil(suite.T(), os.MkdirAll(unreadableDir, 0755))
	assert.Nil(suite.T(), os.Chmod(unreadableDir, 0000))
	defer os.Chmod(unreadableDir, 0755)

	err := suite.scanCommand(func(cmd *Scan) {
		cmd.Exclude = []string{"unreadable-excluded"}
	}).Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.LogContains("Wrote manifest")
}

func (suite *CommandsIntegrationTestSuite) TestScanCommandContinuesPastUnreadableDirectory() {
	suite.writeTestFile("keep/me", helloWorldString)

	unreadableDir := filepath.Join(suite.tempDir, "unreadable")
	assert.Nil(suite.T(), os.MkdirAll(unreadableDir, 0755))
	assert.Nil(suite.T(), os.Chmod(unreadableDir, 0000))
	defer os.Chmod(unreadableDir, 0755)

	cmd := suite.scanCommand()
	err := cmd.Execute([]string{})
	assert.NotNil(suite.T(), err)

	suite.LogContains("unreadable: permission denied")
	suite.LogContains("1 files could not be read.")
	suite.LogContains("Wrote manifest")

	// The rest of the tree should still have been scanned and recorded
	// despite the unreadable subtree.
	f, openErr := os.Open(cmd.WrittenManifestPath)
	assert.Nil(suite.T(), openErr)
	defer f.Close()
	entries, readErr := ReadManifest(f)
	assert.Nil(suite.T(), readErr)
	_, hasKeepMe := entries["keep/me"]
	assert.True(suite.T(), hasKeepMe)
}

func (suite *CommandsIntegrationTestSuite) TestScanCommandWithNameExclusionIsAdditiveToDefaults() {
	suite.writeTestFile("keep/me", helloWorldString)
	suite.writeTestFile("keep/.DS_Store", "junk")
	suite.writeTestFile("keep/custom-junk", "also junk")

	cmd := suite.scanCommand(func(cmd *Scan) {
		cmd.Exclude = []string{"custom-junk"}
	})
	err := cmd.Execute([]string{})
	assert.Nil(suite.T(), err)

	// Get the manifest we just wrote and check its entries directly, since
	// neither excluded name ever shows up as a tracked path to begin with.
	f, openErr := os.Open(cmd.WrittenManifestPath)
	assert.Nil(suite.T(), openErr)
	defer f.Close()
	entries, readErr := ReadManifest(f)
	assert.Nil(suite.T(), readErr)

	_, hasKeepMe := entries["keep/me"]
	_, hasDSStore := entries["keep/.DS_Store"]
	_, hasCustomJunk := entries["keep/custom-junk"]
	assert.True(suite.T(), hasKeepMe)
	assert.False(suite.T(), hasDSStore, "built-in default exclusion (.DS_Store) should still apply")
	assert.False(suite.T(), hasCustomJunk, "--exclude should add to, not replace, the defaults")
}

func (suite *CommandsIntegrationTestSuite) TestScanCommandLogFileAndTruncation() {
	suite.writeTestFile("foo/deleted1", "deleted1")
	suite.writeTestFile("foo/deleted2", "deleted2")
	suite.writeTestFile("foo/deleted3", "deleted3")
	err := suite.scanCommand().Execute([]string{})
	assert.Nil(suite.T(), err)

	suite.deleteTestFile("foo/deleted1")
	suite.deleteTestFile("foo/deleted2")
	suite.deleteTestFile("foo/deleted3")

	suite.clearLog()

	logFile := filepath.Join(suite.tempDir, "full-report.txt")
	err = suite.scanCommand(func(cmd *Scan) {
		cmd.LogFile = logFile
		cmd.Truncate = 1
	}).Execute([]string{})
	assert.NotNil(suite.T(), err)

	suite.LogContains("... and 2 more (see full report)")

	fullReport, readErr := ioutil.ReadFile(logFile)
	assert.Nil(suite.T(), readErr)
	assert.Contains(suite.T(), string(fullReport), "foo/deleted1")
	assert.Contains(suite.T(), string(fullReport), "foo/deleted2")
	assert.Contains(suite.T(), string(fullReport), "foo/deleted3")
	assert.NotContains(suite.T(), string(fullReport), "... and")
}

func TestCommandsIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(CommandsIntegrationTestSuite))
}
