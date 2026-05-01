package pruner

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/typed/key"
	"github.com/NethermindEth/juno/db/typed/prefix"
)

// Rangeable wrappers for the plain typed buckets that are keyed by a bare
// uint64 block number. The wrapper adds a Uint64 prefix layer so the bucket
// exposes DeleteRange/Scan in terms of uint64 bounds. .
var (
	blockHeadersRange = prefix.NewPrefixedBucket(
		core.BlockHeadersByNumberBucket,
		prefix.Prefix(key.Uint64, prefix.End[core.Header]()),
	)
	blockCommitmentsRange = prefix.NewPrefixedBucket(
		core.BlockCommitmentsBucket,
		prefix.Prefix(key.Uint64, prefix.End[core.BlockCommitments]()),
	)
	stateUpdatesRange = prefix.NewPrefixedBucket(
		core.StateUpdatesByBlockNumberBucket,
		prefix.Prefix(key.Uint64, prefix.End[core.StateUpdate]()),
	)
)

// PruneUpto deletes every block strictly below endExclusive. Returns the
// number of blocks actually pruned (which may be less than the full
// window if ctx is cancelled mid-loop).
//
// Resumes automatically. The lower bound of the per-block sweep is
// derived from [earliestBlockNumberByCommitments] — i.e., wherever the
// previous call left off — so two consecutive calls do no overlapping
// per-block work. If a previous call exited mid-loop (ctx cancelled),
// the next call picks up from the same block. If endExclusive is at or
// below the current oldest kept block, the call is a no-op.
//
// Deleted for every block in [oldestKept, endExclusive):
//
//   - block commitments
//   - state update
//   - block transactions and their hash → (block, index) reverse lookups
//   - L1 handler message-hash → tx-hash reverse lookups
//   - contract storage / nonce / class-hash history snapshots
//
// Retained below endExclusive (intentional carve-outs):
//
//  1. Block headers in [endExclusive-BlockHashLag, endExclusive). The
//     get_block_hash_syscall (see [core.BlockHashLag]), run during
//     execution of any block in [endExclusive, endExclusive+BlockHashLag),
//     reads a header by number from this window — pruning it would break
//     the syscall.
//
//  2. The hash → number mapping for endExclusive-1. Resolving
//     StateAtBlockHash(endExclusive.parentHash) needs this single mapping;
//     it is cleaned up by the next PruneUpto call's oldestKept-1 sweep, so
//     at rest exactly one extra mapping survives below endExclusive.
//
// ctx cancellation aborts the per-block loop after the current iteration;
// any work already queued plus the range delete for the partial window is
// still flushed, so the next call resumes from where this one stopped.
//
// Returns (blocksPruned, oldestKept, err): blocksPruned is how many
// blocks this call removed, and oldestKept is the lowest block number
// still present after the call (= blockNum reached by the loop, which
// equals endExclusive on full completion). On the no-op paths
// (empty database, endExclusive ≤ existing oldest), oldestKept reflects
// the unchanged pre-call state — 0 if the database is empty.
func PruneUpto(
	ctx context.Context,
	database db.KeyValueStore,
	endExclusive uint64,
	targetBatchByteSize int,
) (blocksPruned, oldestKept uint64, err error) {
	start, err := earliestBlockNumberByCommitments(database)
	if errors.Is(err, db.ErrKeyNotFound) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	if start >= endExclusive {
		return 0, start, nil
	}

	blockNum, err := pruneHashKeyedUpto(ctx, database, start, endExclusive, targetBatchByteSize)
	if err != nil {
		return 0, 0, err
	}

	batch := database.NewBatch()
	if err := PruneBlockDataUpto(batch, blockNum); err != nil {
		return 0, 0, err
	}
	if err := batch.Write(); err != nil {
		return 0, 0, err
	}

	return blockNum - start, blockNum, nil
}

// pruneHashKeyedUpto deletes the hash-keyed indexes for every block in
// [start, endExclusive), iterating per-block and rotating the batch
// whenever its size exceeds targetBatchByteSize. Also point-deletes the
// carve-out left at start-1 by the previous PruneUpto call. Owns its
// own batch lifecycle and writes the final batch before returning.
//
// Indexes touched:
//
//   - BlockHeaderNumberByHash (skipping endExclusive-1, the carve-out
//     for StateAtBlockHash(endExclusive.parentHash))
//   - TransactionBlockNumbersAndIndicesByHash
//   - L1HandlerTxnHashByMsgHash (only for L1 handler txs)
//   - ContractStorageHistory / ContractNonceHistory / ContractClassHashHistory
//
// Returns the last blockNum reached (= endExclusive on completion, or
// earlier if ctx was cancelled mid-loop).
func pruneHashKeyedUpto(
	ctx context.Context,
	database db.KeyValueStore,
	start,
	endExclusive uint64,
	targetBatchByteSize int,
) (uint64, error) {
	batch := database.NewBatch()
	// Clean up the carve-out left by the previous PruneUpto call.
	if start > 0 {
		header, err := core.GetBlockHeaderByNumber(database, start-1)
		if err != nil {
			return 0, err
		}
		if err := core.DeleteBlockHeaderNumberByHash(batch, header.Hash); err != nil {
			return 0, err
		}
	}

	blockNum := start
	for ; blockNum < endExclusive; blockNum++ {
		if err := ctx.Err(); err != nil {
			break
		}

		su, err := core.GetStateUpdateByBlockNum(database, blockNum)
		if err != nil {
			return 0, err
		}

		// Skip endExclusive-1: its hash→number mapping is the carve-out
		// resolved by StateAtBlockHash(endExclusive.parentHash). Cleaned up
		// by the next PruneUpto call via the start-1 branch above.
		if blockNum != endExclusive-1 {
			if err := core.DeleteBlockHeaderNumberByHash(batch, su.BlockHash); err != nil {
				return 0, err
			}
		}

		if err := deleteTransactionHashReverseLookups(database, batch, blockNum); err != nil {
			return 0, err
		}

		if err := pruneStateHistoryFromUpdate(batch, blockNum, su); err != nil {
			return 0, err
		}

		if batch.Size() >= targetBatchByteSize {
			if err := batch.Write(); err != nil {
				return 0, err
			}
			batch = database.NewBatch()
		}
	}

	return blockNum, batch.Write()
}

// DeleteTransactionHashReverseLookups deletes the hash-keyed indexes that
// point back to a block's transactions:
//
//   - bucket 9: transaction hash → (block number, index)
//   - bucket 24 (only for L1 handler txs): L1 message hash → tx hash
func deleteTransactionHashReverseLookups(
	reader db.KeyValueReader,
	writer db.KeyValueWriter,
	blockNumber uint64,
) error {
	for tx, err := range core.GetTransactionsByBlockNumberIter(reader, blockNumber) {
		if err != nil {
			return err
		}
		txHash := (*felt.TransactionHash)(tx.Hash())
		err := core.TransactionBlockNumbersAndIndicesByHashBucket.Delete(writer, txHash)
		if err != nil {
			return err
		}
		if l1handler, ok := tx.(*core.L1HandlerTransaction); ok {
			err := core.DeleteL1HandlerTxnHashByMsgHash(writer, l1handler.MessageHash())
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// PruneBlockDataUpto prunes the number-keyed block data for every block
// strictly below rangeEndExclusive.
//
// Deleted for every block below rangeEndExclusive:
//
//   - block commitments
//   - state update
//   - block transactions
//   - aggregated bloom filters fully below rangeEndExclusive
//
// Retained below rangeEndExclusive (intentional carve-out):
//
//   - Block headers in [rangeEndExclusive-BlockHashLag, rangeEndExclusive).
//     The get_block_hash_syscall, run during execution of any block in
//     [rangeEndExclusive, rangeEndExclusive+BlockHashLag), reads a header
//     by number from this window — pruning it would break the syscall.
func PruneBlockDataUpto(w db.KeyValueRangeDeleter, rangeEndExclusive uint64) error {
	// When rangeEndExclusive <= BlockHashLag the whole range falls inside
	// the lag carve-out, so no headers may be deleted.
	headerEnd := uint64(0)
	if rangeEndExclusive > core.BlockHashLag {
		headerEnd = rangeEndExclusive - core.BlockHashLag
	}
	err := blockHeadersRange.Prefix().DeleteRange(w, 0, headerEnd)
	if err != nil {
		return err
	}

	err = blockCommitmentsRange.Prefix().DeleteRange(w, 0, rangeEndExclusive)
	if err != nil {
		return err
	}

	err = stateUpdatesRange.Prefix().DeleteRange(w, 0, rangeEndExclusive)
	if err != nil {
		return err
	}

	err = core.BlockTransactionsBucket.Prefix().DeleteRange(w, 0, rangeEndExclusive)
	if err != nil {
		return err
	}

	return pruneAggregatedBloomFiltersUpto(w, rangeEndExclusive)
}

// pruneStateHistoryFromUpdate deletes the state-history entries written
// for a single block. Each entry records the value of a (contract, slot)
// or (contract,) tuple *just before* the block was applied, keyed by block
// number; reads at any later block already use the post-block value, so
// these per-block snapshots are only needed to reconstruct historical
// state at the exact block being pruned. Once we drop that block the
// snapshot is unreachable.
//
// Buckets touched (all keyed by suffix block number):
//
//   - ContractStorageHistory:  contract + slot + block → old slot value
//   - ContractNonceHistory:    contract + block         → old nonce
//   - ContractClassHashHistory: contract + block        → old class hash
//
// stateUpdate is used as the index of which (addr, slot) / addr tuples the
// block touched — only those have history entries to delete.
func pruneStateHistoryFromUpdate(
	w db.KeyValueWriter,
	blockNumber uint64,
	stateUpdate *core.StateUpdate,
) error {
	for addr, storageChanges := range stateUpdate.StateDiff.StorageDiffs {
		for slot := range storageChanges {
			err := core.DeleteContractStorageHistory(w, &addr, &slot, blockNumber)
			if err != nil {
				return err
			}
		}
	}

	for addr := range stateUpdate.StateDiff.Nonces {
		err := core.DeleteContractNonceHistory(w, &addr, blockNumber)
		if err != nil {
			return err
		}
	}

	for addr := range stateUpdate.StateDiff.ReplacedClasses {
		err := core.DeleteContractClassHashHistory(w, &addr, blockNumber)
		if err != nil {
			return err
		}
	}
	return nil
}

// pruneAggregatedBloomFiltersUpto deletes every aggregated bloom filter
// whose entire range lies strictly below rangeEndExclusive.
func pruneAggregatedBloomFiltersUpto(w db.KeyValueRangeDeleter, rangeEndExclusive uint64) error {
	if rangeEndExclusive < core.NumBlocksPerFilter {
		return nil
	}

	// The largest filter boundary at or below rangeEndExclusive — the
	// `from` of the first filter we keep, i.e. the oldest block surviving
	// this prune.
	oldestKept := rangeEndExclusive - rangeEndExclusive%core.NumBlocksPerFilter

	startKey := db.AggregatedBloomFilterKey(0, core.NumBlocksPerFilter-1)
	endKey := db.AggregatedBloomFilterKey(oldestKept, oldestKept+core.NumBlocksPerFilter-1)
	return w.DeleteRange(startKey, endKey)
}

// earliestBlockNumberByCommitments returns the lowest block number
// present in [db.BlockCommitments] — the oldest block still fully
// retained.
//
// BlockCommitments is the source of truth for "oldest kept" because
// the BlockHeadersByNumber carry intentional carve-outs that sit
// below the true retention floor (see [PruneUpto]).
//
// BlockCommitments has no such carve-out, so its lowest entry equals
// oldestKept exactly.
func earliestBlockNumberByCommitments(r db.KeyValueReader) (uint64, error) {
	// Bucket (1 byte) + uint64BE (8 bytes)
	const blockCommitmentsKeyByteSize = 9
	for entry, err := range blockCommitmentsRange.Prefix().Scan(r) {
		if err != nil {
			return 0, err
		}

		if len(entry.Key) != blockCommitmentsKeyByteSize {
			return 0, fmt.Errorf(
				"invalid key size. expected: %v, actual %v",
				blockCommitmentsKeyByteSize,
				len(entry.Key),
			)
		}

		return binary.BigEndian.Uint64(entry.Key[1:9]), nil
	}
	return 0, db.ErrKeyNotFound
}
