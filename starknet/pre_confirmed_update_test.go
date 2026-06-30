package starknet_test

import (
	"bytes"
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

func TestDecodePreConfirmedUpdate(t *testing.T) {
	// The latest tag carries block_number on the wire (the caller is
	// discovering the tip), so the envelope unwraps it into BlockNumber.
	t.Run("latest tag response", func(t *testing.T) {
		t.Run("Full decodes as a new round carrying block_number", func(t *testing.T) {
			raw := loadFeederTestdata(t, "sepolia/preconfirmed/latest/full.json")

			env, err := starknet.DecodePreConfirmedUpdate(bytes.NewReader(raw))
			require.NoError(t, err)
			require.Equal(t, uint64(10936237), env.BlockNumber, "latest endpoint carries block_number")

			full, ok := env.Update.(starknet.PreConfirmedBlock)
			require.True(t, ok, "expected PreConfirmedBlock, got %T", env.Update)
			require.Equal(t, "0x1cbe25d9", full.BlockIdentifier)
			require.Equal(t, "PRE_CONFIRMED", full.Status)
			require.NotZero(t, full.Timestamp)
			require.NotNil(t, full.SequencerAddress)
			require.NotNil(t, full.L1GasPrice)
		})

		t.Run("Delta decodes carrying block_number", func(t *testing.T) {
			// Appended-delta poll: changed=true, no timestamp, carrying the
			// transactions/receipts/state-diffs appended since the known tx count.
			raw := loadFeederTestdata(t, "sepolia/preconfirmed/latest/0x1cbe25d9/3.json")

			env, err := starknet.DecodePreConfirmedUpdate(bytes.NewReader(raw))
			require.NoError(t, err)
			require.Equal(t, uint64(10936237), env.BlockNumber, "latest endpoint carries block_number")

			delta, ok := env.Update.(starknet.PreConfirmedDeltaUpdate)
			require.True(t, ok, "expected PreConfirmedDeltaUpdate, got %T", env.Update)
			require.Equal(t, "0x1cbe25d9", delta.BlockIdentifier)
			require.Len(t, delta.Transactions, 1)
			require.Len(t, delta.Receipts, 1)
			require.Len(t, delta.TransactionStateDiffs, 1)
			// The appended tx/receipt decoded into real, non-nil wire values.
			require.NotNil(t, delta.Transactions[0].Hash)
			require.NotNil(t, delta.Receipts[0].TransactionHash)
		})

		t.Run("NoChange decodes when nothing was appended", func(t *testing.T) {
			raw := loadFeederTestdata(t, "sepolia/preconfirmed/latest/0x1cbe25d9/4.json")

			env, err := starknet.DecodePreConfirmedUpdate(bytes.NewReader(raw))
			require.NoError(t, err)

			_, ok := env.Update.(starknet.PreConfirmedNoChange)
			require.True(t, ok, "expected PreConfirmedNoChange, got %T", env.Update)
		})
	})

	// The explicit-number endpoint omits block_number (the caller already knows
	// the height), so the envelope leaves BlockNumber at zero.
	t.Run("explicit number response", func(t *testing.T) {
		t.Run("Full decodes as a new round omitting block_number", func(t *testing.T) {
			raw := loadFeederTestdata(t, "sepolia/preconfirmed/1781802365/full.json")

			env, err := starknet.DecodePreConfirmedUpdate(bytes.NewReader(raw))
			require.NoError(t, err)
			require.Zero(t, env.BlockNumber, "numbered endpoint omits block_number")

			full, ok := env.Update.(starknet.PreConfirmedBlock)
			require.True(t, ok, "expected PreConfirmedBlock, got %T", env.Update)
			require.Equal(t, "0x1857317c", full.BlockIdentifier)
			require.Equal(t, "PRE_CONFIRMED", full.Status)
			require.NotZero(t, full.Timestamp)
		})

		t.Run("Delta decodes omitting block_number", func(t *testing.T) {
			raw := loadFeederTestdata(t, "sepolia/preconfirmed/1781802365/0x1857317c/2.json")

			env, err := starknet.DecodePreConfirmedUpdate(bytes.NewReader(raw))
			require.NoError(t, err)
			require.Zero(t, env.BlockNumber, "numbered endpoint omits block_number")

			delta, ok := env.Update.(starknet.PreConfirmedDeltaUpdate)
			require.True(t, ok, "expected PreConfirmedDeltaUpdate, got %T", env.Update)
			require.Equal(t, "0x1857317c", delta.BlockIdentifier)
		})

		t.Run("NoChange decodes when nothing was appended", func(t *testing.T) {
			raw := loadFeederTestdata(t, "sepolia/preconfirmed/1781802365/0x1857317c/4.json")

			env, err := starknet.DecodePreConfirmedUpdate(bytes.NewReader(raw))
			require.NoError(t, err)

			_, ok := env.Update.(starknet.PreConfirmedNoChange)
			require.True(t, ok, "expected PreConfirmedNoChange, got %T", env.Update)
		})
	})

	// Variant discrimination and malformed payloads are endpoint-independent.
	t.Run("discrimination and malformed payloads", func(t *testing.T) {
		t.Run("changed absent returns error", func(t *testing.T) {
			_, err := starknet.DecodePreConfirmedUpdate(bytes.NewReader([]byte(`{}`)))
			require.Error(t, err)
		})

		t.Run("changed=false ignores additional fields", func(t *testing.T) {
			// A NoChange response is identified purely by changed=false; any
			// additional fields must be ignored.
			raw := []byte(`{
				"changed": false,
				"block_identifier": "ignored",
				"transactions": [{"transaction_hash": "0x1"}]
			}`)

			env, err := starknet.DecodePreConfirmedUpdate(bytes.NewReader(raw))
			require.NoError(t, err)

			_, ok := env.Update.(starknet.PreConfirmedNoChange)
			require.True(t, ok, "expected PreConfirmedNoChange, got %T", env.Update)
		})

		t.Run("invalid JSON returns error", func(t *testing.T) {
			_, err := starknet.DecodePreConfirmedUpdate(bytes.NewReader([]byte(`{`)))
			require.Error(t, err)
		})

		t.Run("invalid Full payload returns error", func(t *testing.T) {
			// `timestamp` must be a uint64; passing an object surfaces the inner
			// decode error rather than being silently dropped.
			raw := []byte(`{"changed": true, "timestamp": {}}`)
			_, err := starknet.DecodePreConfirmedUpdate(bytes.NewReader(raw))
			require.Error(t, err)
		})

		t.Run("invalid Delta payload returns error", func(t *testing.T) {
			raw := []byte(`{"changed": true, "block_identifier": 7}`)
			_, err := starknet.DecodePreConfirmedUpdate(bytes.NewReader(raw))
			require.Error(t, err)
		})
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
		SequencerAddress:      felt.NewFromUint64[felt.Felt](0xaa),
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
		require.ErrorContains(t, err, "invalid pre_confirmed update type")
	})
}

func TestPreConfirmedBlock_validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*starknet.PreConfirmedBlock)
		wantErr bool
	}{
		{
			name:   "valid",
			mutate: func(*starknet.PreConfirmedBlock) {},
		},
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
		{
			name:   "valid",
			mutate: func(*starknet.PreConfirmedDeltaUpdate) {},
		},
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
