package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
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

func TestFieldsNoHelperReadsStopTheGenerator(t *testing.T) {
	type unknown struct{ Mode uint }
	type ownEncoding struct{ Hash felt.Felt }
	type overridden struct{ Mode uint }
	for name, tt := range map[string]struct {
		reader reader
		err    string
	}{
		"unknown type":     {reader{typ: reflect.TypeFor[unknown](), name: "readX"}, "main.unknown.Mode: no helper reads a uint"},
		"own encoding":     {reader{typ: reflect.TypeFor[ownEncoding](), name: "readX"}, "main.ownEncoding.Hash: felt.Felt has its own encoding"},
		"stale override":   {reader{typ: reflect.TypeFor[overridden](), name: "readX", overrides: map[string]string{"Mode": "", "Gone": ""}}, "names a field it does not have"},
		"overridden field": {reader{typ: reflect.TypeFor[overridden](), name: "readX", overrides: map[string]string{"Mode": ""}}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			err := writeReader(new(strings.Builder), tt.reader, nil)
			if tt.err == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.err)
			}
		})
	}
}
