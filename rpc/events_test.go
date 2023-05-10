package rpc_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalJSON(t *testing.T) {
	jsonString := `{ "from_block": { "block_number": 1 }, "to_block": { "block_number": 1 }, "address": "0x0", "keys": [], "chunk_size": 1 }`
	want := &rpc.EventsArg{
		EventFilter: &rpc.EventFilter{
			FromBlock: &rpc.BlockID{
				Number: 1,
			},
			ToBlock: &rpc.BlockID{
				Number: 1,
			},
			Address: &felt.Zero,
			Keys:    make([]*felt.Felt, 0),
		},
		ResultPageRequest: &rpc.ResultPageRequest{
			ChunkSize: 1,
		},
	}

	got := new(rpc.EventsArg)
	err := got.UnmarshalJSON([]byte(jsonString))

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestUnmarshalJSONErrors(t *testing.T) {
	tests := []struct {
		description string
		json        string
	}{
		{
			description: "well-formed without continuation token",
		},
		{
			description: "missing from_block",
			json:        `{ "to_block": { "block_number": 1 }, "address": "0x0", "keys": [], "chunk_size": 1 }`,
		},
		{
			description: "missing to_block",
			json:        `{ "from_block": { "block_number": 1 }, "address": "0x0", "keys": [], "chunk_size": 1 }`,
		},
		{
			description: "missing address",
			json:        `{ "from_block": { "block_number": 1 }, "to_block": { "block_number": 1 }, "keys": [], "chunk_size": 1 }`,
		},
		{
			description: "missing keys",
			json:        `{ "from_block": { "block_number": 1 }, "to_block": { "block_number": 1 }, "address": "0x0", "chunk_size": 1 }`,
		},
		{
			description: "chunk size less than 1",
			json:        `{ "from_block": { "block_number": 1 }, "to_block": { "block_number": 1 }, "address": "0x0", "keys": [] }`,
		},
	}

	got := new(rpc.EventsArg)
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			original := *got
			err := got.UnmarshalJSON([]byte(tt.json))
			require.Error(t, err)
			// If UnmarshalJSON fails, it shouldn't mutate the EventsArg object.
			assert.Equal(t, original, *got)
		})
	}
}
