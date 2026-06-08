package eth_test

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/l1/eth"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeader_UnmarshalJSON_GethParity(t *testing.T) {
	// Geth Header.MarshalJSON requires Number != nil; everything else can
	// stay zero. Use a non-trivial value that exercises hex digits.
	g := &gethtypes.Header{
		Number:     big.NewInt(0xabcdef1234),
		Difficulty: big.NewInt(1), // also required for marshaling
	}
	raw, err := json.Marshal(g)
	require.NoError(t, err)

	var ours eth.Header
	require.NoError(t, json.Unmarshal(raw, &ours))
	assert.Equal(t, g.Number.Uint64(), uint64(ours.Number))
}

func TestHeader_UnmarshalJSON_ZeroNumber(t *testing.T) {
	g := &gethtypes.Header{
		Number:     big.NewInt(0),
		Difficulty: big.NewInt(0),
	}
	raw, err := json.Marshal(g)
	require.NoError(t, err)

	var ours eth.Header
	require.NoError(t, json.Unmarshal(raw, &ours))
	assert.Equal(t, uint64(0), uint64(ours.Number))
}
