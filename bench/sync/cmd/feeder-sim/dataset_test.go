package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDatasetWriteRead(t *testing.T) {
	dataset := dataset{root: t.TempDir()}
	file := "get_block/5.json.gz"

	_, err := dataset.read(file)
	require.ErrorIs(t, err, fs.ErrNotExist)

	require.NoError(t, dataset.write(file, []byte("first")))
	got, err := dataset.read(file)
	require.NoError(t, err)
	require.Equal(t, []byte("first"), got)

	require.NoError(t, dataset.write(file, []byte("second")))
	got, err = dataset.read(file)
	require.NoError(t, err)
	require.Equal(t, []byte("second"), got)

	entries, err := os.ReadDir(filepath.Join(dataset.root, "get_block"))
	require.NoError(t, err)
	require.Len(t, entries, 1, "temporary files must not linger")
	require.Equal(t, "5.json.gz", entries[0].Name())
}

func TestDatasetWriteRejectsUnwritableRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "file-not-dir")
	require.NoError(t, os.WriteFile(root, nil, 0o600))

	err := dataset{root: root}.write("get_block/5.json.gz", []byte("x"))
	require.Error(t, err)
}

func TestGzipRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		body []byte
	}{
		{"empty", []byte{}},
		{"json", []byte(`{"a": 1}`)},
		{"fixture", readFixture(t, "block", "56377.json")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			compressed, err := gzipBytes(test.body)
			require.NoError(t, err)
			require.NotEqual(t, test.body, compressed)

			got, err := gunzip(compressed)
			require.NoError(t, err)
			require.Equal(t, test.body, got)
		})
	}
}

func TestGunzipRejectsPlainBytes(t *testing.T) {
	_, err := gunzip([]byte("plain text, not gzipped"))
	require.ErrorContains(t, err, "gzip: invalid header")
}

func TestUnmarshalGzipped(t *testing.T) {
	type payload struct {
		Number uint64 `json:"number"`
	}
	tests := []struct {
		name    string
		body    []byte
		want    payload
		wantErr string
	}{
		{"decodes", gzipped(t, []byte(`{"number": 7}`)), payload{Number: 7}, ""},
		{"not gzip", []byte(`{"number": 7}`), payload{}, "gzip: invalid header"},
		{"not json", gzipped(t, []byte(`{"number": `)), payload{}, "unexpected end of JSON input"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := unmarshalGzipped[payload](test.body)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}
