package rpcv10_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	rpc "github.com/NethermindEth/juno/rpc/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockIDUnmarshalJSON(t *testing.T) {
	hash := felt.NewUnsafeFromString[felt.Felt](
		"0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943",
	)

	tests := []struct {
		name     string
		data     string
		expected rpc.BlockID
	}{
		{"latest", `"latest"`, rpc.BlockIDLatest()},
		{"pre_confirmed", `"pre_confirmed"`, rpc.BlockIDPreConfirmed()},
		{"l1_accepted", `"l1_accepted"`, rpc.BlockIDL1Accepted()},
		{"number", `{"block_number":42}`, rpc.BlockIDFromNumber(42)},
		{"number zero", `{"block_number":0}`, rpc.BlockIDFromNumber(0)},
		{"hash", `{"block_hash":"` + hash.String() + `"}`, rpc.BlockIDFromHash(hash)},
		{"leading whitespace", "  \n\t" + `"latest"`, rpc.BlockIDLatest()},
		{
			"hash wins over number",
			`{"block_hash":"` + hash.String() + `","block_number":42}`,
			rpc.BlockIDFromHash(hash),
		},
		{"null hash falls back to number", `{"block_hash":null,"block_number":42}`, rpc.BlockIDFromNumber(42)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var blockID rpc.BlockID
			require.NoError(t, json.Unmarshal([]byte(test.data), &blockID))
			assert.Equal(t, test.expected, blockID)
		})
	}

	errorTests := []struct {
		name string
		data string
	}{
		{"unknown tag", `"pending"`},
		{"empty object", `{}`},
		{"unknown key", `{"block_id":42}`},
		{"bare number", `42`},
		{"array", `[]`},
		{"malformed hash", `{"block_hash":"not a felt"}`},
		{"negative number", `{"block_number":-1}`},
		{"null hash", `{"block_hash":null}`},
		{"null number", `{"block_number":null}`},
	}

	for _, test := range errorTests {
		t.Run("error/"+test.name, func(t *testing.T) {
			var blockID rpc.BlockID
			require.Error(t, json.Unmarshal([]byte(test.data), &blockID))
		})
	}

	// encoding/json rejects malformed input before it reaches the unmarshaler, so call it directly.
	rawErrorTests := []struct {
		name string
		data string
	}{
		{"blank input", "  \n\t"},
		{"unterminated tag", `"latest`},
	}

	for _, test := range rawErrorTests {
		t.Run("error/"+test.name, func(t *testing.T) {
			var blockID rpc.BlockID
			require.Error(t, blockID.UnmarshalJSON([]byte(test.data)))
		})
	}
}
