package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFeederDecode(t *testing.T) {
	plain := []byte(`{"block_number": 1}`)
	tests := []struct {
		name     string
		body     []byte
		encoding string
		wantErr  string
	}{
		{"gzip kept as is", gzipped(t, plain), "gzip", ""},
		{"identity compressed", plain, "identity", ""},
		{"unset compressed", plain, "", ""},
		{"brotli rejected", plain, "br", `unsupported Content-Encoding "br"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := (&feederAPI{}).decode(dataset{}, resource{}, test.body, test.encoding)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}

			require.NoError(t, err)
			unzipped, err := gunzip(got)
			require.NoError(t, err)
			require.Equal(t, plain, unzipped)
		})
	}
}
