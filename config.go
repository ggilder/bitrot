package main

import (
	"path/filepath"
	"strings"
)

var defaultExcludedFiles = []string{
	// Mac OS Finder metadata
	".DS_Store",
	// Mac OS folder icon: "Icon" with ^M at the end
	string([]byte{0x49, 0x63, 0x6f, 0x6e, 0x0d}),
	// VCS folders
	".git",
	".svn",
	// ignore the manifest dir itself
	manifestDirName,
}

// Config for bitrot checks such as file/folder names to exclude.
type Config struct {
	ExcludedFiles []string
}

func DefaultConfig() *Config {
	return &Config{
		ExcludedFiles: defaultExcludedFiles,
	}
}

func (c *Config) isIgnoredPath(path string) bool {
	parts := strings.Split(path, string(filepath.Separator))
	for _, part := range parts {
		for _, ignoredName := range c.ExcludedFiles {
			if part == ignoredName {
				return true
			}
		}
	}
	return false
}
