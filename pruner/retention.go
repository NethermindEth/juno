package pruner

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

// ErrBlockPruned is the sentinel matched by [errors.Is] for any
// [BlockPrunedError]. Use [errors.As] when the requested-block /
// oldest-retained fields are needed.
var ErrBlockPruned = errors.New("block has been pruned")

// BlockPrunedError reports that a requested block has been removed by
// the pruner. OldestRetained is zero when the node has no retained blocks
// (empty database) or when the lookup itself failed.
type BlockPrunedError struct {
	BlockNumber    uint64
	OldestRetained uint64
}

func (e *BlockPrunedError) Error() string {
	if e.OldestRetained == 0 {
		return fmt.Sprintf("block %d is below the node's retention floor", e.BlockNumber)
	}
	return fmt.Sprintf(
		"block %d has been pruned; oldest retained block is %d",
		e.BlockNumber,
		e.OldestRetained,
	)
}

func (e *BlockPrunedError) Is(target error) bool { return target == ErrBlockPruned }

// RequireRetained returns nil if blockNumber is fully retained, otherwise
// a [*BlockPrunedError]. Retention is probed via BlockCommitments — the
// source of truth for "block has not been pruned", since it has no
// carve-out (see [PruneUpto]).
func RequireRetained(r db.KeyValueReader, blockNumber uint64) error {
	_, err := core.GetBlockCommitmentByBlockNum(r, blockNumber)
	if err == nil {
		return nil
	}
	if !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}
	oldest, oldestErr := OldestRetainedBlock(r)
	if oldestErr != nil {
		oldest = 0
	}
	return &BlockPrunedError{BlockNumber: blockNumber, OldestRetained: oldest}
}

// RetentionFloor publishes the lowest block number whose historical state is
// still queryable. The pruner raises it after each prune and readers consult
// it instead reaching to DB. The zero value is unseeded: readers fall back to
// the database probe, preserving correctness for standalone tools that never
// wire a floor. Seed via [NewRetentionFloor].
type RetentionFloor struct {
	// state holds floor+1 so the zero value reads as unseeded.
	state atomic.Uint64
}

// NewRetentionFloor derives the floor from the database: state at
// oldestRetained-1 is still reconstructible from the retained history
// entries (they record pre-block values), matching the hash → number
// carve-out left by [PruneUpto]. An empty database floors at zero.
func NewRetentionFloor(r db.KeyValueReader) (*RetentionFloor, error) {
	oldest, err := OldestRetainedBlock(r)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	f := &RetentionFloor{}
	if oldest > 0 {
		f.raiseTo(oldest - 1)
	} else {
		f.raiseTo(0)
	}
	return f, nil
}

// raiseTo raises the floor to the given value; lower values are ignored so
// concurrent readers never observe the floor moving down.
func (f *RetentionFloor) raiseTo(floor uint64) {
	for {
		cur := f.state.Load()
		if floor+1 <= cur {
			return
		}
		if f.state.CompareAndSwap(cur, floor+1) {
			return
		}
	}
}

// floor returns the current floor and whether it has been seeded.
func (f *RetentionFloor) floor() (uint64, bool) {
	s := f.state.Load()
	return s - 1, s > 0
}

// RequireStateRetainedByBlockNumber checks state retention by number.
func RequireStateRetainedByBlockNumber(
	r db.KeyValueReader,
	floor *RetentionFloor,
	blockNumber uint64,
) error {
	if f, seeded := floor.floor(); seeded {
		if blockNumber < f {
			return db.ErrKeyNotFound
		}
		height, err := core.GetChainHeight(r)
		if err != nil {
			return err
		}
		if blockNumber > height {
			return db.ErrKeyNotFound
		}
		return nil
	}

	hash, err := core.GetBlockHeaderHashByNumber(r, blockNumber)
	if err != nil {
		return err
	}
	_, err = core.GetBlockHeaderNumberByHash(r, hash)
	return err
}

// StateRootIfStateRetainedByBlockNumber returns the global state root for
// blockNumber only if state at that block is queryable.
func StateRootIfStateRetainedByBlockNumber(
	r db.KeyValueReader,
	floor *RetentionFloor,
	blockNumber uint64,
) (*felt.Felt, error) {
	if f, seeded := floor.floor(); seeded {
		if blockNumber < f {
			return nil, db.ErrKeyNotFound
		}
		// The header read doubles as the upper-bound check: headers above
		// the chain height don't exist, and retained headers below the
		// floor (the BlockHashLag carve-out) are already rejected above.
		return core.GetGlobalStateRootByBlockNumber(r, blockNumber)
	}

	hash, stateRoot, err := core.GetBlockHeaderHashAndStateRootByNumber(r, blockNumber)
	if err != nil {
		return nil, err
	}
	if _, err := core.GetBlockHeaderNumberByHash(r, hash); err != nil {
		return nil, err
	}
	return stateRoot, nil
}

// BlockNumberByHashIfStateRetained resolves blockHash to its number, avoiding the full
// header decode (and its heavy fields like EventsBloom) when only the number is needed.
func BlockNumberByHashIfStateRetained(r db.KeyValueReader, blockHash *felt.Felt) (uint64, error) {
	return core.GetBlockHeaderNumberByHash(r, blockHash)
}

// OldestRetainedBlock returns the lowest block number still fully retained,
// found by scanning BlockCommitments — the source of truth, since it has
// no carve-out (see [PruneUpto]). Returns db.ErrKeyNotFound on an empty
// database.
func OldestRetainedBlock(r db.KeyValueReader) (uint64, error) {
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
