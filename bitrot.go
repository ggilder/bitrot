package bitrot

import (
	"path/filepath"
	"strings"
)

type Config struct {
	ExcludedFiles []string
}

func isIgnoredPath(path string, ignored *[]string) bool {
	parts := strings.Split(path, string(filepath.Separator))
	for _, part := range parts {
		for _, ignoredName := range *ignored {
			if part == ignoredName {
				return true
			}
		}
	}
	return false
}
