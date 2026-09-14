package history

import (
	"bytes"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/migration/state/newstate/internal/common"
)

type storageIngestor struct {
	common.BaseIngestor
	scratches []historyScratch
}

var _ pipeline.State[felt.Address, common.Task] = (*storageIngestor)(nil)

func newStorageIngestor(
	sem semaphore.ResourceSemaphore[db.Batch],
	database db.KeyValueReader,
) *storageIngestor {
	return &storageIngestor{
		BaseIngestor: common.NewBaseIngestor(sem, database),
		scratches:    make([]historyScratch, common.IngestorCount),
	}
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
func (i *storageIngestor) Run(index int, addr felt.Address, outputs chan<- common.Task) error {
	addrFelt := (*felt.Felt)(&addr)
	task := &i.Tasks[index]
	scratch := &i.scratches[index]

	prefix := fillAddressKey(scratch.deprecatedPrefix[:], db.DeprecatedContractStorageHistory, &addr)
	deprecatedHistoryIt, err := i.Database.NewIterator(prefix, true)
	if err != nil {
		return fmt.Errorf("opening deprecated storage history of %s: %w", addrFelt.String(), err)
	}
	defer deprecatedHistoryIt.Close()
	if !deprecatedHistoryIt.First() {
		return nil
	}

	leafPrefix := fillLeafPrefix(scratch.leafPrefix[:], &addr)
	headStorageTrieIt, err := i.Database.NewIterator(leafPrefix, true)
	if err != nil {
		return fmt.Errorf("opening head storage trie of %s: %w", addrFelt.String(), err)
	}
	defer headStorageTrieIt.Close()
	leafValid := headStorageTrieIt.First()

	// key trails one deprecated row behind the iterator: it names the slot and
	// block whose post-value the row being read supplies.
	key := scratch.key[:storageHistoryKeyLen]
	if err := fillHistoryKeyFrom(
		key, db.ContractStorageHistory, deprecatedHistoryIt.UncopiedKey(),
	); err != nil {
		return fmt.Errorf("%s: %w", addrFelt.String(), err)
	}

	for deprecatedHistoryIt.Next() {
		// Checked here rather than left to fillHistoryKeyFrom because the slot
		// comparison below slices rowKey first.
		rowKey := deprecatedHistoryIt.UncopiedKey()
		if len(rowKey) != storageHistoryKeyLen {
			return fmt.Errorf(
				"%s: malformed deprecated storage history key: length %d, want %d",
				addrFelt.String(), len(rowKey), storageHistoryKeyLen,
			)
		}

		if bytes.Equal(rowKey[slotOffset:blockOffset], key[slotOffset:blockOffset]) {
			// Same slot: this row's stored pre-value is the value in effect
			// after the block the key still trails on.
			value, valueErr := deprecatedHistoryIt.UncopiedValue()
			if valueErr != nil {
				return fmt.Errorf("reading deprecated storage of %s: %w", addrFelt.String(), valueErr)
			}
			if err := task.Batch.Put(key, value); err != nil {
				return err
			}
		} else {
			// Slot changed: the row key names was the outgoing slot's last
			// change, so close that slot from the head trie before the
			// register moves on.
			err := writeSlotTail(task.Batch, key, headStorageTrieIt, &leafValid)
			if err != nil {
				return fmt.Errorf("%s: %w", addrFelt.String(), err)
			}
		}

		if err := fillHistoryKeyFrom(key, db.ContractStorageHistory, rowKey); err != nil {
			return fmt.Errorf("%s: %w", addrFelt.String(), err)
		}
		task.EntryCount++
		if err := i.Flush(task, outputs); err != nil {
			return err
		}
	}

	// The final slot's last entry, which no successor can supply.
	if err := writeSlotTail(
		task.Batch, key, headStorageTrieIt, &leafValid,
	); err != nil {
		return fmt.Errorf("%s: %w", addrFelt.String(), err)
	}
	task.EntryCount++
	if err := i.Flush(task, outputs); err != nil {
		return err
	}

	task.CompletedAddrs++
	return nil
}

// writeSlotTail writes the last entry for the slot key names: the slot's
// head-trie leaf value, or zero if the slot was dropped from the trie. On a
// leaf match it advances headIt past it.
func writeSlotTail(
	batch db.Batch,
	key []byte,
	headIt db.Iterator,
	leafValid *bool,
) error {
	slot := key[slotOffset:blockOffset]
	if !*leafValid || !bytes.Equal(headIt.UncopiedKey()[leafPrefixLen:], slot) {
		return batch.Put(key, zeroValue[:])
	}

	// A leaf node is encoded value-first, so its value needs no decoding.
	leaf, err := headIt.UncopiedValue()
	if err != nil {
		return fmt.Errorf("reading head storage leaf: %w", err)
	}
	if len(leaf) < felt.Bytes {
		return fmt.Errorf(
			"malformed head storage leaf: len %d, want at least %d", len(leaf), felt.Bytes,
		)
	}

	// Write before advancing: the leaf slice dies on the next cursor move.
	if err := batch.Put(key, leaf[:felt.Bytes]); err != nil {
		return err
	}
	*leafValid = headIt.Next()
	return nil
}
