package bitrot

import (
	"path/filepath"
	"strings"
)

// Config for bitrot checks such as file/folder names to exclude.
type Config struct {
	ExcludedFiles []string
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

// General helper functions
func check(e error) {
	if e != nil {
		panic(e)
	}
}
