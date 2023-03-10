package rpc_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockId(t *testing.T) {
	tests := map[string]struct {
		blockIdJson     string
		expectedBlockId rpc.BlockId
	}{
		"latest": {
			blockIdJson: "\"latest\"",
			expectedBlockId: rpc.BlockId{
				Latest: true,
			},
		},
		"pending": {
			blockIdJson: "\"pending\"",
			expectedBlockId: rpc.BlockId{
				Pending: true,
			},
		},
		"number": {
			blockIdJson: `{ "block_number" : 123123 }`,
			expectedBlockId: rpc.BlockId{
				Number: 123123,
			},
		},
		"hash": {
			blockIdJson: `{ "block_hash" : "0x123" }`,
			expectedBlockId: rpc.BlockId{
				Hash: new(felt.Felt).SetUint64(0x123),
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var blockId rpc.BlockId
			require.NoError(t, blockId.UnmarshalJSON([]byte(test.blockIdJson)))
			assert.Equal(t, test.expectedBlockId, blockId)
		})
	}

	failingTests := map[string]struct {
		blockIdJson string
	}{
		"unknown tag": {
			blockIdJson: "\"unknown tag\"",
		},
		"an empyt json object": {
			blockIdJson: "{  }",
		},
		"a json list": {
			blockIdJson: "[  ]",
		},
		"cannot parse number": {
			blockIdJson: `{ "block_number" : asd }`,
		},
		"cannot parse hash": {
			blockIdJson: `{ "block_hash" : asd }`,
		},
	}
	for name, test := range failingTests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var blockId rpc.BlockId
			assert.Error(t, blockId.UnmarshalJSON([]byte(test.blockIdJson)))
		})
	}
}
