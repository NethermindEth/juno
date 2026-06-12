package reedsolomon_test

import (
	"bytes"
	"crypto/rand"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/consensus/propeller/reedsolomon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeData(t *testing.T) {
	requireEqualLength := func(t *testing.T, fragments [][]byte) {
		length := len(fragments[0])
		for i := range fragments {
			require.Len(t, fragments[i], length)
		}
	}

	requireEqualPrefix := func(t *testing.T, expected []byte, fragments [][]byte) {
		actual := bytes.Join(fragments, nil)
		require.Truef(
			t,
			bytes.HasPrefix(actual, expected),
			"expected to get prefix: %s in %s",
			expected,
			actual,
		)
	}

	largeData := make([]byte, 10*1024)
	_, err := rand.Read(largeData)
	require.NoError(t, err)

	successTests := []struct {
		name    string
		data    []byte
		numData int
		parity  int
	}{
		{
			name:    "success",
			data:    []byte("A journey of a thousands shards begins with a single byte"),
			numData: 5,
			parity:  3,
		},
		{
			name:    "single data shard and single parity",
			data:    []byte("some data"),
			numData: 1,
			parity:  1,
		},
		{
			name:    "large data",
			data:    largeData,
			numData: 8,
			parity:  4,
		},
		{
			name:    "above 256 total shards",
			data:    largeData,
			numData: 200,
			parity:  100,
		},
	}
	for _, tc := range successTests {
		t.Run(tc.name, func(t *testing.T) {
			shards, err := reedsolomon.EncodeData(tc.data, tc.numData, tc.parity)
			require.NoError(t, err)
			require.Len(t, shards, tc.numData+tc.parity)
			requireEqualLength(t, shards)
			requireEqualPrefix(t, tc.data, shards)
		})
	}

	errorTests := []struct {
		name        string
		data        []byte
		numData     int
		parity      int
		errContains string
	}{
		{
			name:        "empty data",
			data:        []byte{},
			numData:     5,
			parity:      3,
			errContains: "received empty data",
		},
		{
			name:        "zero data shards",
			data:        []byte("data"),
			numData:     0,
			parity:      3,
			errContains: "creating Reed-Solomon encoder",
		},
		{
			name:        "negative parity",
			data:        []byte("data"),
			numData:     5,
			parity:      -1,
			errContains: "creating Reed-Solomon encoder",
		},
		{
			name:        "exceeds max shard count",
			data:        []byte("data"),
			numData:     40000,
			parity:      40000,
			errContains: "creating Reed-Solomon encoder",
		},
	}
	for _, tc := range errorTests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := reedsolomon.EncodeData(tc.data, tc.numData, tc.parity)
			require.ErrorContains(t, err, tc.errContains)
		})
	}
}

func TestRecoverData(t *testing.T) {
	encode := func(t *testing.T, data []byte, numData, parity int) [][]byte {
		t.Helper()
		shards, err := reedsolomon.EncodeData(data, numData, parity)
		require.NoError(t, err)
		return shards
	}

	buildDataShards := func(t *testing.T, original [][]byte, missingData ...int) [][]byte {
		t.Helper()
		dataShards := make([][]byte, len(original))
		for i := range original {
			if slices.Contains(missingData, i) {
				continue
			}
			dataShards[i] = make([]byte, len(original[i]))
			copy(dataShards[i], original[i])
		}
		return dataShards
	}

	requireEqualShards := func(t *testing.T, expected [][]byte, actual [][]byte) {
		t.Helper()
		for i := range expected {
			assert.Equalf(
				t, expected[i], actual[i],
				"at index %d, expected: %s, actual: %s",
				i, expected[i], actual[i],
			)
		}
	}

	successTests := []struct {
		name       string
		data       []byte
		numData    int
		parity     int
		missingIdx []int
	}{
		{
			name:    "no missing shards",
			data:    []byte("nothing is missing here"),
			numData: 4,
			parity:  2,
		},
		{
			name:       "missing parity shards",
			data:       []byte("recover parity shards"),
			numData:    4,
			parity:     3,
			missingIdx: []int{5, 6, 7},
		},
		{
			name:       "missing data shards within parity limit",
			data:       []byte("recover data shards from parity"),
			numData:    5,
			parity:     3,
			missingIdx: []int{0, 2, 4},
		},
		{
			name:       "missing mixed data and parity shards",
			data:       []byte("mixed missing shards scenario"),
			numData:    5,
			parity:     4,
			missingIdx: []int{1, 3, 5, 6},
		},
	}
	for _, tc := range successTests {
		t.Run(tc.name, func(t *testing.T) {
			expected := encode(t, tc.data, tc.numData, tc.parity)
			dataShards := buildDataShards(t, expected, tc.missingIdx...)

			recovered, err := reedsolomon.RecoverData(dataShards, tc.numData, tc.parity)
			require.NoError(t, err)
			requireEqualShards(t, expected, recovered)
		})
	}

	errorTests := []struct {
		name        string
		data        []byte
		numData     int
		parity      int
		missingIdx  []int
		errContains string
	}{
		{
			name:        "too many missing shards",
			data:        []byte("too many shards gone"),
			numData:     4,
			parity:      2,
			missingIdx:  []int{0, 1, 4},
			errContains: "recovering the data shards:",
		},
		{
			name:        "empty shards slice",
			numData:     4,
			parity:      2,
			errContains: "no data shards provided",
		},
	}
	for _, tc := range errorTests {
		t.Run(tc.name, func(t *testing.T) {
			var shards [][]byte
			if tc.data != nil {
				expected := encode(t, tc.data, tc.numData, tc.parity)
				shards = buildDataShards(t, expected, tc.missingIdx...)
			}

			recovered, err := reedsolomon.RecoverData(shards, tc.numData, tc.parity)
			require.Nil(t, recovered)
			require.ErrorContains(t, err, tc.errContains)
		})
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		numData int
		parity  int
		nilIdxs []int // indices to nil out before recovery
	}{
		{
			name:    "small data, lose 1 data shard",
			data:    []byte("round trip test"),
			numData: 4, parity: 2,
			nilIdxs: []int{0},
		},
		{
			name:    "medium data, lose max shards",
			data:    bytes.Repeat([]byte("abcdefghij"), 100),
			numData: 5, parity: 3,
			nilIdxs: []int{1, 3, 6},
		},
		{
			name:    "single byte",
			data:    []byte{0xff},
			numData: 2, parity: 1,
			nilIdxs: []int{0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			shards, err := reedsolomon.EncodeData(tc.data, tc.numData, tc.parity)
			require.NoError(t, err)

			for _, idx := range tc.nilIdxs {
				shards[idx] = nil
			}

			recovered, err := reedsolomon.RecoverData(shards, tc.numData, tc.parity)
			require.NoError(t, err)

			joined := bytes.Join(recovered[:tc.numData], nil)
			assert.Equal(t, tc.data, joined[:len(tc.data)])
		})
	}
}
