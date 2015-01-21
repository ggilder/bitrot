package main

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestPathStringExtractsPath(t *testing.T) {
	cmd := Generate{PathWrapper: PathWrapped{Path: "/foo/bar"}}
	path := cmd.PathString()
	assert.Equal(t, "/foo/bar", path)
}

func TestPathStringIsAbsolute(t *testing.T) {
	dir, _ := os.Getwd()
	cmd := Generate{PathWrapper: PathWrapped{Path: "."}}
	path := cmd.PathString()
	assert.Equal(t, dir, path)
}

// TODO write integration test for `generate` command?
