package main

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
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

// TODO write integration test for `generate` command?
