package checksum_generator

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(dir, name, content string) string {
	testFile := filepath.Join(dir, name)
	err := ioutil.WriteFile(testFile, []byte(content), 0400)
	check(err)
	return testFile
}

func TestStuff(t *testing.T) {
	// FileChecksum()
	var tempDir, err = ioutil.TempDir("", "checksum")
	check(err)

	defer os.RemoveAll(tempDir)

	testFile := writeTestFile(tempDir, "foo", "hello! world\n")
	sum := FileChecksum(testFile)

	correctSum := "87b3fe7479c73ae4246dbe8081550f52e2cf9e59"
	if sum != correctSum {
		t.Fatalf("expected checksum %s; got %s", correctSum, sum)
	}
}
