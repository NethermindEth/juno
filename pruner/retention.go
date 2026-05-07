package pruner

import (
	"encoding/binary"
	"errors"
	"fmt"

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
	return fmt.Sprintf("block %d has been pruned; oldest retained block is %d",
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

// HeaderByNumberIfStateRetained returns the header for blockNumber only if
// state at that block is queryable. Use this for state access only; for
// block-level data (transactions, receipts, etc.) use [RequireRetained] + the
// plain core accessors instead — the two checks diverge at oldestKept-1,
// where state is preserved by carve-out but block-level data is not.
// See [PruneUpto] for the carve-out semantics. Returns db.ErrKeyNotFound
// when state at blockNumber is not available.
func HeaderByNumberIfStateRetained(r db.KeyValueReader, blockNumber uint64) (*core.Header, error) {
	header, err := core.GetBlockHeaderByNumber(r, blockNumber)
	if err != nil {
		return nil, err
	}
	if _, err := core.GetBlockHeaderNumberByHash(r, header.Hash); err != nil {
		return nil, err
	}
	return header, nil
}

// HeaderByHashIfStateRetained returns the header for blockHash only if
// state at that block is queryable. Use this for state access only; for
// block-level data use [RequireRetained] + the plain core accessors instead.
// See [PruneUpto] for the carve-out semantics. Returns db.ErrKeyNotFound
// when state at blockHash is not available.
func HeaderByHashIfStateRetained(r db.KeyValueReader, blockHash *felt.Felt) (*core.Header, error) {
	return core.GetBlockHeaderByHash(r, blockHash)
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
