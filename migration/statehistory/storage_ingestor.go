package statehistory

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
