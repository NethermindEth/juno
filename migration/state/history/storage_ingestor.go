package history

import (
	"bytes"
	"context"
	"fmt"

	"github.com/NethermindEth/juno/core/deprecatedstate"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
)

type storageIngestor struct {
	baseIngestor
}

var _ pipeline.State[*felt.Felt, task] = (*storageIngestor)(nil)

func newStorageIngestor(
	ctx context.Context,
	sem semaphore.ResourceSemaphore[db.Batch],
	database db.KeyValueReader,
) *storageIngestor {
	return &storageIngestor{baseIngestor: newBaseIngestor(ctx, sem, database)}
}

// Run migrates the per-slot storage history of a single contract.
//
// Legend: Bₙ = block at which the n-th change to a slot happened. preXₙ
// is the value of slot X before Bₙ (= what the deprecated layout stores
// at [X, Bₙ]); headX is the slot's current value, read from the head
// storage trie. The deprecated layout writes nothing at deploy — the
// pre-deploy value (0) is implicit in the first change entry. The new
// layout stores the same number of entries per slot, just shifted to
// post-values. For one slot:
//
//	block  │ deprecated[slotA] │ new[slotA]
//	───────┼───────────────────┼───────────
//	  B₁   │  0                │ preA₁
//	  B₂   │  preA₁            │ preA₂
//	  B₃   │  preA₂            │ headA
//	───────┼───────────────────┼───────────
//	  > B₃ │  head trie leaf   │ headA (last entry — self-contained)
//	          for slotA          ← deprecated must reach into the head
//	                               storage trie for any block past the
//	                               last change
//
// For each deprecated entry the post-value comes from one of:
//
//   - the *next* deprecated entry, when it's on the same slot — its stored
//     pre-value is exactly this block's post-value;
//   - the head storage trie leaf for that slot, when there is no next
//     deprecated entry on the same slot;
//   - felt.Zero, when there is no head leaf for the slot (the slot was
//     eventually zeroed out and dropped from the trie).
//
// Both the deprecated history and the head trie are sorted by raw slot
// bytes, so the ingestor walks them in lockstep — the head-trie iterator
// advances only when its current leaf matches the slot just resolved:
//
//	deprecated history       head trie         new history
//	─────────────────────    ─────────────     ─────────────────────────
//	[slotA, B₁..B₃]    ──→   [slotA] = headA   [slotA, B₁..B₃] last uses headA
//	[slotB, B₁..B₂]    ──→   (no leaf)         [slotB, B₁..B₂] last uses 0
//	                          ← slotB was set                   (slotB was zeroed
//	                            and later zeroed                 at B₂)
//	[slotC, B₁]        ──→   [slotC] = headC   [slotC, B₁] = headC
//
// Contracts with no deprecated storage history are skipped; deprecated
// rows are deleted at the end of the run via DeleteRange.
func (i *storageIngestor) Run(index int, addr *felt.Felt, outputs chan<- task) error {
	t := &i.tasks[index]

	addrBytes := addr.Marshal()
	deprecatedPrefix := db.DeprecatedContractStorageHistory.Key(addrBytes)

	deprecatedHistoryIt, err := i.database.NewIterator(deprecatedPrefix, true)
	if err != nil {
		return fmt.Errorf("storage: open deprecated iter(%s): %w", addr, err)
	}
	defer deprecatedHistoryIt.Close()
	if !deprecatedHistoryIt.First() {
		return nil
	}

	leafPrefix := db.ContractStorage.Key(addrBytes)
	leafPrefix = append(leafPrefix, deprecatedstate.ContractStorageTrieHeight)

	headStorageTrieIt, err := i.database.NewIterator(leafPrefix, true)
	if err != nil {
		return fmt.Errorf("storage: open leaf iter(%s): %w", addr, err)
	}
	defer headStorageTrieIt.Close()
	leafValid := headStorageTrieIt.First()

	for {
		slot, block, err := parseStorageKey(deprecatedHistoryIt.Key(), deprecatedPrefix)
		if err != nil {
			return fmt.Errorf("storage: parse key(%s): %w", addr, err)
		}

		successorSlot, successorValue, hasNext, err := peekSuccessor(
			deprecatedHistoryIt,
			deprecatedPrefix,
			addr,
		)
		if err != nil {
			return err
		}

		historyValue, advanced, err := resolveHistoryValue(
			headStorageTrieIt,
			leafPrefix,
			addr,
			&slot,
			hasNext,
			successorSlot,
			successorValue,
			leafValid,
		)
		if err != nil {
			return err
		}
		if advanced {
			leafValid = headStorageTrieIt.Next()
		}

		err = state.WriteStorageHistory(
			t.batch,
			addr,
			&slot,
			block,
			&historyValue,
		)
		if err != nil {
			return err
		}
		t.entryCount++
		if err := i.flush(t, outputs); err != nil {
			return err
		}

		if !hasNext {
			break
		}
	}

	if err := t.batch.DeleteRange(deprecatedPrefix, dbutils.UpperBound(deprecatedPrefix)); err != nil {
		return fmt.Errorf("storage: DeleteRange deprecated(%s): %w", addr, err)
	}
	t.completedAddrs++
	return nil
}

// peekSuccessor advances the deprecated-history iterator. If a next entry
// exists, returns its (slot, value, true); otherwise returns (_, _, false).
func peekSuccessor(
	it db.Iterator,
	prefix []byte,
	addr *felt.Felt,
) (felt.Felt, felt.Felt, bool, error) {
	if !it.Next() {
		return felt.Felt{}, felt.Felt{}, false, nil
	}
	slot, _, err := parseStorageKey(it.Key(), prefix)
	if err != nil {
		return felt.Felt{}, felt.Felt{}, false, fmt.Errorf(
			"storage: parse successor key(%s): %w", addr, err,
		)
	}
	rawValue, err := it.Value()
	if err != nil {
		return felt.Felt{}, felt.Felt{}, false, fmt.Errorf(
			"storage: read successor value(%s, slot=%s): %w", addr, &slot, err,
		)
	}
	return slot, felt.FromBytes[felt.Felt](rawValue), true, nil
}

// resolveHistoryValue decides the value to install at the current entry: the
// successor's value when both are on the same slot, otherwise the head-trie
// leaf (when the iterator is positioned on this slot), otherwise zero. Returns
// advanced=true when the head-trie iterator should be advanced by the caller.
func resolveHistoryValue(
	headIt db.Iterator,
	leafPrefix []byte,
	addr, slot *felt.Felt,
	hasSuccessor bool,
	successorSlot, successorValue felt.Felt,
	leafValid bool,
) (value felt.Felt, advanced bool, err error) {
	if hasSuccessor && successorSlot == *slot {
		return successorValue, false, nil
	}
	if !leafValid {
		return felt.Felt{}, false, nil
	}
	if !bytes.Equal(headIt.Key()[len(leafPrefix):], slot.Marshal()) {
		return felt.Felt{}, false, nil
	}
	raw, err := headIt.Value()
	if err != nil {
		return felt.Felt{}, false, fmt.Errorf(
			"storage: leaf(%s, slot=%s): %w", addr, slot, err,
		)
	}
	var node trie.Node
	if err := node.UnmarshalBinary(raw); err != nil {
		return felt.Felt{}, false, fmt.Errorf(
			"storage: decode leaf(%s, slot=%s): %w", addr, slot, err,
		)
	}
	return *node.Value, true, nil
}
