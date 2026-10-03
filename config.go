package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
)

const (
	configDir        = ".bitrot"
	configStorageDir = "manifests"
)

// TODO should ignored files and directories be handled separately?
var defaultExcludedNames = []string{
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
	// ExcludedNames are file/directory basenames to exclude wherever they
	// occur in the tree (e.g. ".DS_Store").
	ExcludedNames []string
	// ExcludedPrefixes are paths, relative to the directory being scanned,
	// to exclude along with everything under them (e.g. "Downloads" or
	// "snapshots/weekly").
	ExcludedPrefixes []string
	Dir              string
	manifestStorage  *ManifestStorage
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
		ExcludedNames: defaultExcludedNames,
		Dir:           filepath.Join(basedir, configDir),
	}
}

// isIgnoredPath reports whether relPath, a path relative to the directory
// being scanned, should be excluded. The root of the scan itself (relPath
// == ".") is never excluded.
func (c *Config) isIgnoredPath(relPath string) bool {
	if relPath == "." {
		return false
	}

	base := filepath.Base(relPath)
	for _, ignoredName := range c.ExcludedNames {
		if base == ignoredName {
			return true
		}
	}

	for _, prefix := range c.ExcludedPrefixes {
		prefix = filepath.Clean(prefix)
		if relPath == prefix || strings.HasPrefix(relPath, prefix+string(filepath.Separator)) {
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
