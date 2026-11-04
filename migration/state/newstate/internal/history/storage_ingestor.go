package history

import (
	"bytes"
	"fmt"

	//nolint:staticcheck,nolintlint // old state layout
	"github.com/NethermindEth/juno/core/deprecatedstate"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/migration/state/newstate/internal/common"
)

type storageIngestor struct {
	common.BaseIngestor
	floor uint64
}

var _ pipeline.State[felt.Address, common.Task] = (*storageIngestor)(nil)

func newStorageIngestor(
	sem semaphore.ResourceSemaphore[db.Batch],
	database db.KeyValueReader,
	floor uint64,
) *storageIngestor {
	return &storageIngestor{BaseIngestor: common.NewBaseIngestor(sem, database), floor: floor}
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
//   - zero, when there is no head leaf for the slot (the slot was
//     eventually zeroed out and dropped from the trie).
//
// Both the deprecated history and the head trie are sorted by raw slot
// bytes, so the ingestor walks them in lockstep. Each entry is written once
// the next one is read, since the next pre-value on the same slot is its
// post-value; a slot's last entry takes its head leaf instead:
//
//	deprecated history       head trie         new history
//	─────────────────────    ─────────────     ─────────────────────────
//	[slotA, B₁..B₃]    ──→   [slotA] = headA   [slotA, B₁..B₃] last uses headA
//	[slotB, B₁..B₂]    ──→   (no leaf)         [slotB, B₁..B₂] last uses 0
//	                          ← slotB was set                   (slotB was zeroed
//	                            and later zeroed                 at B₂)
//	[slotC, B₁]        ──→   [slotC] = headC   [slotC, B₁] = headC
//
// Each nonzero slot gets an entry at the floor with its value there: the first
// pre-value, or the head leaf when no history is left. Unpruned, it is always zero.
// Deprecated rows are wiped with the whole bucket once the phase completes.
func (i *storageIngestor) Run(index int, addr felt.Address, outputs chan<- common.Task) error {
	addrFelt := (*felt.Felt)(&addr)
	addrBytes := addrFelt.Marshal()
	//nolint:staticcheck,nolintlint // old state layout
	historyPrefix := db.DeprecatedContractStorageHistory.Key(addrBytes)

	historyIt, err := i.Database.NewIterator(historyPrefix, true)
	if err != nil {
		return fmt.Errorf("storage: open deprecated iter(%s): %w", addrFelt, err)
	}
	defer historyIt.Close()

	//nolint:staticcheck,nolintlint // old state layout
	leafPrefix := db.DeprecatedContractStorage.Key(addrBytes)
	leafPrefix = append(leafPrefix, deprecatedstate.ContractStorageTrieHeight)

	leafIt, err := i.Database.NewIterator(leafPrefix, true)
	if err != nil {
		return fmt.Errorf("storage: open leaf iter(%s): %w", addrFelt, err)
	}
	defer leafIt.Close()

	w := storageWalk{
		storageIngestor: i,
		t:               &i.Tasks[index],
		outputs:         outputs,
		addr:            addrFelt,
		leaves:          leafIt,
		leafPrefix:      leafPrefix,
	}
	if err := w.run(historyIt, historyPrefix); err != nil {
		return err
	}
	w.t.CompletedAddrs++
	return nil
}

// storageWalk migrates one contract's storage.
type storageWalk struct {
	*storageIngestor
	t          *common.Task
	outputs    chan<- common.Task
	addr       *felt.Felt
	leaves     db.Iterator
	leafPrefix []byte
	leafValid  bool
}

func (w *storageWalk) run(history db.Iterator, prefix []byte) error {
	hasHistory := history.First()
	if !hasHistory {
		if migrated, err := w.migrated(); err != nil || migrated {
			return err
		}
	}
	w.leafValid = w.leaves.First()

	var slot *felt.Felt
	var block uint64
	for valid := hasHistory; valid; valid = history.Next() {
		nextSlot, nextBlock, err := parseStorageKey(history.Key(), prefix)
		if err != nil {
			return fmt.Errorf("storage: parse key(%s): %w", w.addr, err)
		}
		rawValue, err := history.Value()
		if err != nil {
			return fmt.Errorf("storage: read value(%s, slot=%s): %w", w.addr, &nextSlot, err)
		}
		valueBefore := felt.FromBytes[felt.Felt](rawValue)

		if slot != nil && *slot == nextSlot {
			if err := w.write(slot, block, &valueBefore); err != nil {
				return err
			}
		} else {
			if err := w.endSlot(slot, block); err != nil {
				return err
			}
			if err := w.startSlot(&nextSlot, &valueBefore); err != nil {
				return err
			}
		}
		slot, block = &nextSlot, nextBlock
	}
	if err := w.endSlot(slot, block); err != nil {
		return err
	}
	return w.writeLeavesBefore(nil)
}

// migrated reports a rerun after the wipe, which must skip: head leaves at the floor would
// overwrite history. Leaves are written in slot order, so only a written last leaf means done.
func (w *storageWalk) migrated() (bool, error) {
	if !w.leaves.Last() {
		return false, nil
	}
	lastSlot := felt.FromBytes[felt.Felt](w.leafSlot())
	return hasEntries(w.Database, db.ContractStorageHistoryKey(w.addr, &lastSlot))
}

// startSlot writes the leaves before nextSlot at the floor, then nextSlot's value at the floor.
func (w *storageWalk) startSlot(nextSlot, floorValue *felt.Felt) error {
	if err := w.writeLeavesBefore(nextSlot); err != nil {
		return err
	}
	if floorValue.IsZero() {
		return nil
	}
	return w.write(nextSlot, w.floor, floorValue)
}

// endSlot writes the slot's last entry: its head leaf, or zero when it has none.
func (w *storageWalk) endSlot(slot *felt.Felt, block uint64) error {
	if slot == nil {
		return nil
	}
	var head felt.Felt
	if w.leafValid && bytes.Equal(w.leafSlot(), slot.Marshal()) {
		var err error
		if head, err = decodeLeaf(w.leaves, w.addr, slot); err != nil {
			return err
		}
		w.leafValid = w.leaves.Next()
	}
	return w.write(slot, block, &head)
}

// writeLeavesBefore writes leaves below slot (all if nil) at the floor: those slots never changed.
func (w *storageWalk) writeLeavesBefore(slot *felt.Felt) error {
	var slotBytes []byte
	if slot != nil {
		slotBytes = slot.Marshal()
	}
	for ; w.leafValid; w.leafValid = w.leaves.Next() {
		if slot != nil && bytes.Compare(w.leafSlot(), slotBytes) >= 0 {
			return nil
		}
		leafSlot := felt.FromBytes[felt.Felt](w.leafSlot())
		leafValue, err := decodeLeaf(w.leaves, w.addr, &leafSlot)
		if err != nil {
			return err
		}
		if err := w.write(&leafSlot, w.floor, &leafValue); err != nil {
			return err
		}
	}
	return nil
}

func (w *storageWalk) leafSlot() []byte {
	return w.leaves.Key()[len(w.leafPrefix):]
}

func (w *storageWalk) write(slot *felt.Felt, block uint64, value *felt.Felt) error {
	if err := state.WriteStorageHistory(w.t.Batch, w.addr, slot, block, value); err != nil {
		return err
	}
	w.t.EntryCount++
	return w.Flush(w.t, w.outputs)
}

// decodeLeaf returns the value of the head-trie leaf it is on.
func decodeLeaf(it db.Iterator, addr, slot *felt.Felt) (felt.Felt, error) {
	raw, err := it.Value()
	if err != nil {
		return felt.Felt{}, fmt.Errorf("storage: leaf(%s, slot=%s): %w", addr, slot, err)
	}
	var node trie.Node
	if err := node.UnmarshalBinary(raw); err != nil {
		return felt.Felt{}, fmt.Errorf("storage: decode leaf(%s, slot=%s): %w", addr, slot, err)
	}
	return *node.Value, nil
}
