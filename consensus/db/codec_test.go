package db_test

import (
	"testing"

	consensusdb "github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/wal"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

// TestWALRecordsRoundTripThroughReopen verifies that persisted WAL records
// survive a store reopen unchanged across all entry kinds and edge branches.
func TestWALRecordsRoundTripThroughReopen(t *testing.T) {
	height := types.Height(1)
	round := types.Round(1)
	value := felt.FromUint64[starknet.Value](1)
	valueHash := value.Hash()

	start := wal.Start(height)
	proposalWithValue := &starknet.WALProposal{
		MessageHeader: starknet.MessageHeader{Height: height, Round: round, Sender: walAddress(1)},
		ValidRound:    1,
		Value:         &value,
	}
	proposalWithoutValue := &starknet.WALProposal{
		MessageHeader: starknet.MessageHeader{Height: height, Round: round, Sender: walAddress(2)},
		ValidRound:    -1,
		Value:         nil,
	}
	prevoteWithID := &starknet.WALPrevote{
		MessageHeader: starknet.MessageHeader{Height: height, Round: round, Sender: walAddress(3)},
		ID:            &valueHash,
	}
	prevoteNilID := &starknet.WALPrevote{
		MessageHeader: starknet.MessageHeader{Height: height, Round: round, Sender: walAddress(4)},
		ID:            nil,
	}
	precommitWithID := &starknet.WALPrecommit{
		MessageHeader: starknet.MessageHeader{Height: height, Round: round, Sender: walAddress(5)},
		ID:            &valueHash,
	}
	precommitNilID := &starknet.WALPrecommit{
		MessageHeader: starknet.MessageHeader{Height: height, Round: round, Sender: walAddress(6)},
		ID:            nil,
	}
	timeout := &starknet.WALTimeout{Height: height, Round: round, Step: types.StepPrecommit}

	entries := []starknet.WALEntry{
		&start,
		proposalWithValue,
		proposalWithoutValue,
		prevoteWithID,
		prevoteNilID,
		precommitWithID,
		precommitNilID,
		timeout,
	}

	walStore, testDB, dbPath := newTestTendermintWALStore(t)
	writeAndFlushEntries(t, walStore, entries)

	walStore, testDB = reopenTestTendermintWALStore(t, walStore, testDB, dbPath)
	t.Cleanup(func() {
		require.NoError(t, walStore.Close())
		require.NoError(t, testDB.Close())
	})

	assertLoadedEntries(t, walStore, entries)
}

func TestWALRecordDecodeRejectsMalformedPayloads(t *testing.T) {
	start := wal.Start(types.Height(1))
	valid, err := consensusdb.EncodeWALRecordPayload(&start)
	require.NoError(t, err)

	tests := map[string][]byte{
		"empty":         {},
		"unknown kind":  {0xFF},
		"truncated":     valid[:len(valid)-1],
		"trailing byte": append(append([]byte(nil), valid...), 0x00),
	}

	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			require.Error(t, consensusdb.DecodeWALRecordPayload(payload))
		})
	}
}

func walAddress(v uint64) starknet.Address {
	return felt.FromUint64[starknet.Address](v)
}
