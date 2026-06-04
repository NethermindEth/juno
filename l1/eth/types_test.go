package eth_test

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/l1/eth"
	gethcommon "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gethLogFixture builds a representative geth Log that mirrors what an
// eth_getLogs response carries — all fields populated so the JSON round
// trip exercises every field shape our HexU64 / HexBytes codecs must handle.
func gethLogFixture() *gethtypes.Log {
	const (
		topicSig = "0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b"
		topicIdx = "0x0000000000000000000000005474c8d22d1a3c3e3e1b2cb1e3a8c5a7a8f5e3a1"
		txHash   = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		blkHash  = "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	)
	return &gethtypes.Log{
		Address: gethcommon.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4"),
		Topics: []gethcommon.Hash{
			gethcommon.HexToHash(topicSig),
			gethcommon.HexToHash(topicIdx),
		},
		Data:        []byte{0x01, 0x02, 0x03, 0x04, 0xab, 0xcd, 0xef},
		BlockNumber: 0x123abc,
		TxHash:      gethcommon.HexToHash(txHash),
		TxIndex:     5,
		BlockHash:   gethcommon.HexToHash(blkHash),
		Index:       7,
		Removed:     true,
	}
}

func TestLog_UnmarshalJSON_GethParity(t *testing.T) {
	g := gethLogFixture()
	raw, err := json.Marshal(g)
	require.NoError(t, err)

	var ours eth.Log
	require.NoError(t, json.Unmarshal(raw, &ours))

	require.Len(t, ours.Topics, len(g.Topics))
	for i := range g.Topics {
		assert.Equal(t, g.Topics[i].Bytes(), ours.Topics[i].Bytes(), "topic[%d]", i)
	}
	assert.Equal(t, g.Data, []byte(ours.Data))
	assert.Equal(t, g.BlockNumber, uint64(ours.BlockNumber))
	assert.Equal(t, g.Removed, ours.Removed)
}

func TestLog_UnmarshalJSON_EmptyData(t *testing.T) {
	// "data": "0x" is a valid empty payload per the JSON-RPC spec.
	g := gethLogFixture()
	g.Data = nil
	raw, err := json.Marshal(g)
	require.NoError(t, err)

	var ours eth.Log
	require.NoError(t, json.Unmarshal(raw, &ours))
	assert.Empty(t, []byte(ours.Data))
}

func TestLog_UnmarshalJSON_ZeroBlockNumber(t *testing.T) {
	g := gethLogFixture()
	g.BlockNumber = 0
	raw, err := json.Marshal(g)
	require.NoError(t, err)

	var ours eth.Log
	require.NoError(t, json.Unmarshal(raw, &ours))
	assert.Equal(t, uint64(0), uint64(ours.BlockNumber))
}

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

func TestReceipt_UnmarshalJSON_GethParity(t *testing.T) {
	const (
		txHash  = "0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
		blkHash = "0xdddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	)
	g := &gethtypes.Receipt{
		Type:              gethtypes.DynamicFeeTxType,
		Status:            1,
		CumulativeGasUsed: 21000,
		Logs:              []*gethtypes.Log{gethLogFixture(), gethLogFixture()},
		TxHash:            gethcommon.HexToHash(txHash),
		GasUsed:           21000,
		EffectiveGasPrice: big.NewInt(1_000_000_000),
		BlockHash:         gethcommon.HexToHash(blkHash),
		BlockNumber:       big.NewInt(0x123abc),
		TransactionIndex:  3,
	}
	raw, err := json.Marshal(g)
	require.NoError(t, err)

	var ours eth.Receipt
	require.NoError(t, json.Unmarshal(raw, &ours))
	require.Len(t, ours.Logs, len(g.Logs))
	for i := range g.Logs {
		assert.Equal(t, g.Logs[i].BlockNumber, uint64(ours.Logs[i].BlockNumber),
			"log[%d].BlockNumber", i)
		assert.Equal(t, g.Logs[i].Data, []byte(ours.Logs[i].Data), "log[%d].Data", i)
		assert.Equal(t, g.Logs[i].Removed, ours.Logs[i].Removed, "log[%d].Removed", i)
		require.Len(t, ours.Logs[i].Topics, len(g.Logs[i].Topics))
		for j := range g.Logs[i].Topics {
			assert.Equal(t, g.Logs[i].Topics[j].Bytes(), ours.Logs[i].Topics[j].Bytes(),
				"log[%d].topic[%d]", i, j)
		}
	}
}

func TestReceipt_UnmarshalJSON_NoLogs(t *testing.T) {
	const txHash = "0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	g := &gethtypes.Receipt{
		Status:            1,
		CumulativeGasUsed: 21000,
		GasUsed:           21000,
		EffectiveGasPrice: big.NewInt(0),
		BlockNumber:       big.NewInt(1),
		TxHash:            gethcommon.HexToHash(txHash),
	}
	raw, err := json.Marshal(g)
	require.NoError(t, err)

	var ours eth.Receipt
	require.NoError(t, json.Unmarshal(raw, &ours))
	assert.Empty(t, ours.Logs)
}

// TestHexCodecs_ErrorPaths exercises the HexU64 / HexBytes error branches
// indirectly through Log, ensuring malformed wire input is rejected rather
// than producing zero values silently.
func TestHexCodecs_ErrorPaths(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{"bad blockNumber prefix", `{"topics":[],"data":"0x","blockNumber":"1234","removed":false}`},
		{"bad blockNumber empty", `{"topics":[],"data":"0x","blockNumber":"0x","removed":false}`},
		{
			"bad blockNumber overflow",
			`{"topics":[],"data":"0x","blockNumber":"0x10000000000000000","removed":false}`,
		},
		{"bad blockNumber hex", `{"topics":[],"data":"0x","blockNumber":"0xZZ","removed":false}`},
		// JSON-RPC "quantity" must be minimally encoded — leading zeros are invalid.
		{
			"bad blockNumber leading zero",
			`{"topics":[],"data":"0x","blockNumber":"0x01","removed":false}`,
		},
		{
			"bad blockNumber leading zeros",
			`{"topics":[],"data":"0x","blockNumber":"0x00","removed":false}`,
		},
		{"bad data prefix", `{"topics":[],"data":"00","blockNumber":"0x0","removed":false}`},
		{"bad data odd length", `{"topics":[],"data":"0x0","blockNumber":"0x0","removed":false}`},
		{"bad data hex", `{"topics":[],"data":"0xZZ","blockNumber":"0x0","removed":false}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var l eth.Log
			err := json.Unmarshal([]byte(tc.json), &l)
			assert.Error(t, err, "input: %s", tc.json)
		})
	}
}
