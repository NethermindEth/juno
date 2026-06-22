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
	t.Run("changed absent returns error", func(t *testing.T) {
		var env starknet.PreConfirmedUpdateEnvelope
		require.Error(t, json.Unmarshal([]byte(`{}`), &env))
	})

	t.Run("changed=true with timestamp decodes as Full (new round)", func(t *testing.T) {
		// Real new-endpoint Full response: "changed": true with timestamp.
		raw := loadFeederTestdata(t, "sepolia-integration/pre_confirmed_delta/11252240.json")

		var env starknet.PreConfirmedUpdateEnvelope
		require.NoError(t, json.Unmarshal(raw, &env))

		full, ok := env.Update.(starknet.PreConfirmedBlock)
		require.True(t, ok, "expected PreConfirmedBlock, got %T", env.Update)
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

		delta, ok := env.Update.(starknet.PreConfirmedDeltaUpdate)
		require.True(t, ok, "expected PreConfirmedDeltaUpdate, got %T", env.Update)
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

		delta, ok := env.Update.(starknet.PreConfirmedDeltaUpdate)
		require.True(t, ok, "expected PreConfirmedDeltaUpdate, got %T", env.Update)
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

func nonEmptyTx() starknet.Transaction {
	return starknet.Transaction{Hash: felt.NewRandom[felt.Felt]()}
}

// validBlock returns a PreConfirmedBlock that passes validate(), so individual
// tests can flip a single field to exercise one failure path at a time.
func validBlock() starknet.PreConfirmedBlock {
	return starknet.PreConfirmedBlock{
		BlockIdentifier:       "abc123",
		Transactions:          []starknet.Transaction{nonEmptyTx()},
		Receipts:              []*starknet.TransactionReceipt{{}},
		TransactionStateDiffs: []*starknet.StateDiff{{}},
		Status:                "PRE_CONFIRMED",
		Version:               "0.14.0",
		SequencerAddress:      new(felt.Felt).SetUint64(0xaa),
		L1GasPrice:            &starknet.GasPrice{},
		L2GasPrice:            &starknet.GasPrice{},
		L1DataGasPrice:        &starknet.GasPrice{},
	}
}

// validDelta returns a PreConfirmedDeltaUpdate that passes validate().
func validDelta() starknet.PreConfirmedDeltaUpdate {
	return starknet.PreConfirmedDeltaUpdate{
		BlockIdentifier:       "abc123",
		Transactions:          []starknet.Transaction{nonEmptyTx()},
		Receipts:              []*starknet.TransactionReceipt{{}},
		TransactionStateDiffs: []*starknet.StateDiff{{}},
	}
}

func TestPreConfirmedUpdateEnvelope_Validate(t *testing.T) {
	t.Run("NoChange is always valid", func(t *testing.T) {
		env := &starknet.PreConfirmedUpdateEnvelope{Update: starknet.PreConfirmedNoChange{}}
		err := env.Validate()
		require.NoError(t, err)
	})

	t.Run("valid Block passes", func(t *testing.T) {
		env := &starknet.PreConfirmedUpdateEnvelope{Update: validBlock()}
		err := env.Validate()
		require.NoError(t, err)
	})

	t.Run("valid Delta passes", func(t *testing.T) {
		env := &starknet.PreConfirmedUpdateEnvelope{Update: validDelta()}
		err := env.Validate()
		require.NoError(t, err)
	})

	t.Run("invalid Block propagates validate error", func(t *testing.T) {
		b := validBlock()
		b.Status = "ACCEPTED_ON_L2"
		env := &starknet.PreConfirmedUpdateEnvelope{Update: b}
		err := env.Validate()
		require.Error(t, err)
	})

	t.Run("invalid Delta propagates validate error", func(t *testing.T) {
		d := validDelta()
		d.Transactions = nil
		env := &starknet.PreConfirmedUpdateEnvelope{Update: d}
		err := env.Validate()
		require.Error(t, err)
	})

	t.Run("nil Update hits the default branch", func(t *testing.T) {
		// A zero-value envelope has a nil Update interface, which falls through
		// the type switch to the default error branch.
		env := &starknet.PreConfirmedUpdateEnvelope{}
		err := env.Validate()
		require.Error(t, err)
	})
}

func TestPreConfirmedBlock_validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*starknet.PreConfirmedBlock)
		wantErr bool
	}{
		{name: "valid", mutate: func(*starknet.PreConfirmedBlock) {}},
		{
			name:    "missing block_identifier",
			mutate:  func(b *starknet.PreConfirmedBlock) { b.BlockIdentifier = "" },
			wantErr: true,
		},
		{
			name:    "wrong status",
			mutate:  func(b *starknet.PreConfirmedBlock) { b.Status = "ACCEPTED_ON_L2" },
			wantErr: true,
		},
		{
			name:    "missing version",
			mutate:  func(b *starknet.PreConfirmedBlock) { b.Version = "" },
			wantErr: true,
		},
		{
			name:    "missing sequencer_address",
			mutate:  func(b *starknet.PreConfirmedBlock) { b.SequencerAddress = nil },
			wantErr: true,
		},
		{
			name:    "missing l1_gas_price",
			mutate:  func(b *starknet.PreConfirmedBlock) { b.L1GasPrice = nil },
			wantErr: true,
		},
		{
			name:    "missing l2_gas_price",
			mutate:  func(b *starknet.PreConfirmedBlock) { b.L2GasPrice = nil },
			wantErr: true,
		},
		{
			name:    "missing l1_data_gas_price",
			mutate:  func(b *starknet.PreConfirmedBlock) { b.L1DataGasPrice = nil },
			wantErr: true,
		},
		{
			name: "mismatched lengths",
			mutate: func(b *starknet.PreConfirmedBlock) {
				b.Receipts = nil
			},
			wantErr: true,
		},
		{
			name: "empty transaction",
			mutate: func(b *starknet.PreConfirmedBlock) {
				b.Transactions = []starknet.Transaction{{}}
			},
			wantErr: true,
		},
		{
			name: "nil receipt",
			mutate: func(b *starknet.PreConfirmedBlock) {
				b.Receipts = []*starknet.TransactionReceipt{nil}
			},
			wantErr: true,
		},
		{
			name: "nil state diff",
			mutate: func(b *starknet.PreConfirmedBlock) {
				b.TransactionStateDiffs = []*starknet.StateDiff{nil}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := validBlock()
			tt.mutate(&b)
			// validate() is unexported; drive it through the exported Validate().
			err := (&starknet.PreConfirmedUpdateEnvelope{Update: b}).Validate()
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestPreConfirmedDeltaUpdate_validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*starknet.PreConfirmedDeltaUpdate)
		wantErr bool
	}{
		{name: "valid", mutate: func(*starknet.PreConfirmedDeltaUpdate) {}},
		{
			name:    "missing block_identifier",
			mutate:  func(d *starknet.PreConfirmedDeltaUpdate) { d.BlockIdentifier = "" },
			wantErr: true,
		},
		{
			name:    "zero transactions",
			mutate:  func(d *starknet.PreConfirmedDeltaUpdate) { d.Transactions = nil },
			wantErr: true,
		},
		{
			name: "mismatched lengths",
			mutate: func(d *starknet.PreConfirmedDeltaUpdate) {
				d.Transactions = []starknet.Transaction{nonEmptyTx(), nonEmptyTx()}
			},
			wantErr: true,
		},
		{
			name: "empty transaction",
			mutate: func(d *starknet.PreConfirmedDeltaUpdate) {
				d.Transactions = []starknet.Transaction{{}}
			},
			wantErr: true,
		},
		{
			name: "nil receipt",
			mutate: func(d *starknet.PreConfirmedDeltaUpdate) {
				d.Receipts = []*starknet.TransactionReceipt{nil}
			},
			wantErr: true,
		},
		{
			name: "nil state diff",
			mutate: func(d *starknet.PreConfirmedDeltaUpdate) {
				d.TransactionStateDiffs = []*starknet.StateDiff{nil}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := validDelta()
			tt.mutate(&d)
			err := (&starknet.PreConfirmedUpdateEnvelope{Update: d}).Validate()
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
		})
	}
}

// AsUpdate produces a PreConfirmedBlock that shares the legacy block's data
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

	full, ok := b.AsUpdate().(starknet.PreConfirmedBlock)
	require.True(t, ok, "AsUpdate must return PreConfirmedBlock, got %T", b.AsUpdate())
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
