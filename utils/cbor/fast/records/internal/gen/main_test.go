package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// A core type that gained or lost a field, or a changed entry of known, leaves the checked-in
// readers behind what the generator writes now.
func TestGeneratedFilesAreFresh(t *testing.T) {
	src, err := generate()
	require.NoError(t, err)
	onDisk, err := os.ReadFile(filepath.Join("..", "..", output))
	require.NoError(t, err)
	if !bytes.Equal(src, onDisk) {
		t.Errorf("%s is stale: run go generate in utils/cbor/fast/records", output)
	}
}
