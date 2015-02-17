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
	"testing"
	"time"
)

func TestPathStringExtractsPath(t *testing.T) {
	args := PathArguments{Path: "/foo/bar"}
	path := args.PathString()
	assert.Equal(t, "/foo/bar", path)
}

func TestPathStringIsAbsolute(t *testing.T) {
	dir, _ := os.Getwd()
	args := PathArguments{Path: "."}
	path := args.PathString()
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
	check(os.Chtimes(testFile, stat.ModTime(), stat.ModTime()))
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

func TestCommandsIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(CommandsIntegrationTestSuite))
}
