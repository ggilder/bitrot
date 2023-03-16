package main

import (
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
)

const (
	configDir        = ".bitrot"
	configStorageDir = "manifests"
)

// TODO should ignored files and directories be handled separately?
var defaultExcludedFiles = []string{
	// Mac OS Finder metadata
	".DS_Store",
	// Mac OS folder icon: "Icon" with ^M at the end
	string([]byte{0x49, 0x63, 0x6f, 0x6e, 0x0d}),
	// VCS folders
	".git",
	".svn",
	// Synology filesystem metadata
	"@eaDir",
	"@tmp",
	// Dropbox cache files
	".dropbox.cache",
	// ignore our own configuration
	configDir,
}

// TODO test this file more granularly (currently integration tested in bitrot_test)

// Config for bitrot checks such as file/folder names to exclude.
type Config struct {
	ExcludedFiles   []string
	Dir             string
	manifestStorage *ManifestStorage
}

func DefaultConfig() *Config {
	basedir, err := homedir.Dir()
	if err != nil {
		basedir, err = os.Getwd()
		if err != nil {
			// it's drastic but... come on
			panic(err)
		}
	}
	return &Config{
		ExcludedFiles: defaultExcludedFiles,
		Dir:           filepath.Join(basedir, configDir),
	}
}

func (c *Config) isIgnoredPath(path string) bool {
	base := filepath.Base(path)
	for _, ignoredName := range c.ExcludedFiles {
		if base == ignoredName {
			return true
		}
	}
	return false
}

func (c *Config) ManifestStorage() *ManifestStorage {
	if c.manifestStorage == nil {
		c.manifestStorage = NewManifestStorage(filepath.Join(c.Dir, configStorageDir))
	}
	return c.manifestStorage
}
