package main

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func roundTrip[T comparable](t *testing.T, name string, value T) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		encoded, err := encode(value)
		require.NoError(t, err)

		values := url.Values{}
		for name, field := range encoded {
			values.Set(name, field)
		}
		decoded, err := decode[T](values)
		require.NoError(t, err)
		require.Equal(t, value, decoded)
	})
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	roundTrip(t, "empty", struct{}{})
	roundTrip(t, "block key", blockKey{BlockNumber: 56377})
	roundTrip(t, "class key", classKey{ClassHash: sierraHash})
	roundTrip(t, "header only", headerOnly{HeaderOnly: true})
	roundTrip(t, "with block and signature", withBlockAndSignature{IncludeBlock: true})
	roundTrip(t, "at latest", atLatest{BlockNumber: "latest"})
}

func TestEncode(t *testing.T) {
	tests := []struct {
		name string
		got  func() (map[string]string, error)
		want map[string]string
	}{
		{"empty", func() (map[string]string, error) { return encode(struct{}{}) }, map[string]string{}},
		{
			"block key",
			func() (map[string]string, error) { return encode(blockKey{BlockNumber: 7}) },
			map[string]string{"blockNumber": "7"},
		},
		{"fixed flags", func() (map[string]string, error) {
			return encode(withBlockAndSignature{IncludeBlock: true, IncludeSignature: true})
		}, map[string]string{"includeBlock": "true", "includeSignature": "true"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.got()
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

type decodeCase[T comparable] struct {
	name    string
	query   string
	want    T
	wantErr bool
}

func runDecodeCases[T comparable](t *testing.T, cases []decodeCase[T]) {
	t.Helper()
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			values, err := url.ParseQuery(test.query)
			require.NoError(t, err)

			got, err := decode[T](values)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestDecode(t *testing.T) {
	t.Run("block key", func(t *testing.T) {
		runDecodeCases(t, []decodeCase[blockKey]{
			{"number", "blockNumber=12", blockKey{BlockNumber: 12}, false},
			{"first value wins", "blockNumber=1&blockNumber=2", blockKey{BlockNumber: 1}, false},
			{"missing is zero", "", blockKey{}, false},
			{"extra keys ignored", "blockNumber=3&headerOnly=true", blockKey{BlockNumber: 3}, false},
			{"not a number", "blockNumber=latest", blockKey{}, true},
			{"negative", "blockNumber=-1", blockKey{}, true},
		})
	})
	t.Run("header only", func(t *testing.T) {
		runDecodeCases(t, []decodeCase[headerOnly]{
			{"true", "headerOnly=true", headerOnly{HeaderOnly: true}, false},
			{"false", "headerOnly=false", headerOnly{}, false},
			{"missing", "", headerOnly{}, false},
			{"not a bool", "headerOnly=maybe", headerOnly{}, true},
		})
	})
}
