package starknet_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/stretchr/testify/require"
)

// loadFeederTestdata returns the raw bytes of a feeder testdata fixture
// relative to the clients/feeder/testdata directory.
func loadFeederTestdata(t *testing.T, relPath string) []byte {
	t.Helper()
	full := filepath.Join("..", "clients", "feeder", "testdata", relPath)
	data, err := os.ReadFile(full)
	require.NoError(t, err, "load %s", full)
	return data
}

func TestPreConfirmedUpdateEnvelope_UnmarshalJSON(t *testing.T) {
	t.Run("changed absent decodes as Full (legacy upstream)", func(t *testing.T) {
		// Real legacy-endpoint response: no "changed" field, full block payload.
		raw := loadFeederTestdata(t, "sepolia-integration/pre_confirmed/1204672.json")

		var env starknet.PreConfirmedUpdateEnvelope
		require.NoError(t, json.Unmarshal(raw, &env))

		full, ok := env.Update.(starknet.PreConfirmedFull)
		require.True(t, ok, "expected PreConfirmedFull, got %T", env.Update)
		require.Equal(t, "PRE_CONFIRMED", full.Status)
		require.NotZero(t, full.Timestamp)
		require.NotEmpty(t, full.Version)
		require.Equal(t, starknet.Blob, full.L1DAMode)
		require.NotNil(t, full.SequencerAddress)
		require.NotNil(t, full.L1GasPrice)
		require.NotNil(t, full.L2GasPrice)
		require.NotNil(t, full.L1DataGasPrice)
	})

	t.Run("changed=true with timestamp decodes as Full (new round)", func(t *testing.T) {
		// Real new-endpoint Full response: "changed": true with timestamp.
		raw := loadFeederTestdata(t, "sepolia-integration/pre_confirmed_delta/11252240.json")

		var env starknet.PreConfirmedUpdateEnvelope
		require.NoError(t, json.Unmarshal(raw, &env))

		full, ok := env.Update.(starknet.PreConfirmedFull)
		require.True(t, ok, "expected PreConfirmedFull, got %T", env.Update)
		require.NotEmpty(t, full.BlockIdentifier, "new-round Full must carry an identifier")
		require.Equal(t, "PRE_CONFIRMED", full.Status)
		require.NotZero(t, full.Timestamp)
		require.NotNil(t, full.SequencerAddress)
		require.NotNil(t, full.L1GasPrice)
	})

	// NOTE: NoChange and Delta-without-timestamp paths still use inline JSON
	// because no real-wire fixtures for them exist in clients/feeder/testdata.
	// If/when fixtures are added (e.g. sepolia-integration/pre_confirmed_delta
	// captures of an actual no-change / appended-delta poll), these should
	// move to loadFeederTestdata.

	t.Run("changed=false decodes as NoChange", func(t *testing.T) {
		raw := []byte(`{"changed": false}`)

		var env starknet.PreConfirmedUpdateEnvelope
		require.NoError(t, json.Unmarshal(raw, &env))

		_, ok := env.Update.(starknet.PreConfirmedNoChange)
		require.True(t, ok, "expected PreConfirmedNoChange, got %T", env.Update)
	})

	t.Run("changed=false ignores additional fields", func(t *testing.T) {
		// A NoChange response is identified purely by changed=false; any
		// additional fields must be ignored.
		raw := []byte(`{
			"changed": false,
			"block_identifier": "ignored",
			"transactions": [{"transaction_hash": "0x1"}]
		}`)

		var env starknet.PreConfirmedUpdateEnvelope
		require.NoError(t, json.Unmarshal(raw, &env))

		_, ok := env.Update.(starknet.PreConfirmedNoChange)
		require.True(t, ok, "expected PreConfirmedNoChange, got %T", env.Update)
	})

	t.Run("changed=true without timestamp decodes as Delta", func(t *testing.T) {
		raw := []byte(`{
			"changed": true,
			"block_identifier": "abc123",
			"transactions": [],
			"transaction_receipts": [],
			"transaction_state_diffs": []
		}`)

		var env starknet.PreConfirmedUpdateEnvelope
		require.NoError(t, json.Unmarshal(raw, &env))

		delta, ok := env.Update.(starknet.PreConfirmedDelta)
		require.True(t, ok, "expected PreConfirmedDelta, got %T", env.Update)
		require.Equal(t, "abc123", delta.BlockIdentifier)
		require.Empty(t, delta.Transactions)
	})

	t.Run("Delta carries appended txs and receipts (carved from real Full)", func(t *testing.T) {
		// We don't have a real Delta wire fixture. To get the tx/receipt/state-diff
		// payloads onto the real wire path anyway, splice them at the JSON-bytes
		// level: read a real Full, lift the txs/receipts/state-diffs arrays
		// out as json.RawMessage, take their tail, and re-emit inside a Delta
		// envelope (changed=true, block_identifier, no timestamp). The actual
		// wire-shape of each tx/receipt/state-diff is the server's, only the
		// Delta-specific wrapper is synthesised.
		fullRaw := loadFeederTestdata(t, "sepolia-integration/pre_confirmed_delta/11252240.json")

		var fullFields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(fullRaw, &fullFields))

		var blockIdentifier string
		require.NoError(t, json.Unmarshal(fullFields["block_identifier"], &blockIdentifier))

		var txs, receipts, stateDiffs []json.RawMessage
		require.NoError(t, json.Unmarshal(fullFields["transactions"], &txs))
		require.NoError(t, json.Unmarshal(fullFields["transaction_receipts"], &receipts))
		require.NoError(t, json.Unmarshal(fullFields["transaction_state_diffs"], &stateDiffs))
		require.Equal(t, len(txs), len(receipts))
		require.Equal(t, len(txs), len(stateDiffs))
		require.GreaterOrEqual(t, len(txs), 2,
			"fixture must have >=2 txs to carve a non-trivial Delta")

		// Take the last 2 entries as the "appended" Delta tail.
		k := len(txs) - 2
		deltaJSON, err := json.Marshal(map[string]any{
			"changed":                 true,
			"block_identifier":        blockIdentifier,
			"transactions":            txs[k:],
			"transaction_receipts":    receipts[k:],
			"transaction_state_diffs": stateDiffs[k:],
		})
		require.NoError(t, err)

		var env starknet.PreConfirmedUpdateEnvelope
		require.NoError(t, json.Unmarshal(deltaJSON, &env))

		delta, ok := env.Update.(starknet.PreConfirmedDelta)
		require.True(t, ok, "expected PreConfirmedDelta, got %T", env.Update)
		require.Equal(t, blockIdentifier, delta.BlockIdentifier)
		require.Len(t, delta.Transactions, 2)
		require.Len(t, delta.Receipts, 2)
		require.Len(t, delta.TransactionStateDiffs, 2)
		// Each carved tx decoded into a real, non-nil Transaction.
		require.NotNil(t, delta.Transactions[0].Hash)
		require.NotNil(t, delta.Transactions[1].Hash)
		require.NotNil(t, delta.Receipts[0].TransactionHash)
		require.NotNil(t, delta.Receipts[1].TransactionHash)
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		var env starknet.PreConfirmedUpdateEnvelope
		require.Error(t, json.Unmarshal([]byte(`{`), &env))
	})

	t.Run("invalid Full payload returns error", func(t *testing.T) {
		// `timestamp` must be a uint64; passing an object surfaces the inner
		// decode error rather than being silently dropped.
		raw := []byte(`{"timestamp": {}}`)
		var env starknet.PreConfirmedUpdateEnvelope
		require.Error(t, json.Unmarshal(raw, &env))
	})

	t.Run("invalid Delta payload returns error", func(t *testing.T) {
		raw := []byte(`{"changed": true, "block_identifier": 7}`)
		var env starknet.PreConfirmedUpdateEnvelope
		require.Error(t, json.Unmarshal(raw, &env))
	})
}

// AsUpdate produces a PreConfirmedFull that shares the legacy block's data
// and carries the synthetic "LegacyAPI" identifier downstream code uses to
// detect a legacy source.
func TestPreConfirmedBlock_AsUpdate(t *testing.T) {
	b := &starknet.DeprecatedPreConfirmedBlock{
		Status:                "PRE_CONFIRMED",
		Timestamp:             7,
		Version:               "0.14.0",
		L1DAMode:              starknet.Blob,
		Transactions:          []starknet.Transaction{{}, {}},
		Receipts:              []*starknet.TransactionReceipt{{}, {}},
		TransactionStateDiffs: []*starknet.StateDiff{{}, {}},
		SequencerAddress:      new(felt.Felt).SetUint64(0xaa),
		L1GasPrice:            &starknet.GasPrice{},
		L2GasPrice:            &starknet.GasPrice{},
		L1DataGasPrice:        &starknet.GasPrice{},
	}

	full, ok := b.AsUpdate().(starknet.PreConfirmedFull)
	require.True(t, ok, "AsUpdate must return PreConfirmedFull, got %T", b.AsUpdate())
	require.Equal(t, "LegacyAPI", full.BlockIdentifier)
	require.Equal(t, b.Status, full.Status)
	require.Equal(t, b.Timestamp, full.Timestamp)
	require.Equal(t, b.Version, full.Version)
	require.Equal(t, b.L1DAMode, full.L1DAMode)
	require.Len(t, full.Transactions, 2)
	require.Len(t, full.Receipts, 2)
	require.Len(t, full.TransactionStateDiffs, 2)
	require.Same(t, b.SequencerAddress, full.SequencerAddress)
	require.Same(t, b.L1GasPrice, full.L1GasPrice)
	require.Same(t, b.L2GasPrice, full.L2GasPrice)
	require.Same(t, b.L1DataGasPrice, full.L1DataGasPrice)
}
