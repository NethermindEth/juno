package history

import (
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/migration/state/newstate/internal/common"
)

type nonceIngestor struct {
	common.BaseIngestor
	floor uint64
}

var _ pipeline.State[felt.Address, common.Task] = (*nonceIngestor)(nil)

func newNonceIngestor(
	sem semaphore.ResourceSemaphore[db.Batch],
	database db.KeyValueReader,
	floor uint64,
) *nonceIngestor {
	return &nonceIngestor{BaseIngestor: common.NewBaseIngestor(sem, database), floor: floor}
}

// Run migrates the nonce history of a single contract.
//
// Legend: Bₙ = block at which the n-th nonce change happened. Nₙ = the
// nonce active *after* Bₙ; the deploy nonce is always 0 and is *not*
// written to the deprecated history — its presence is implicit in the
// pre-value of the first change entry. The new layout stores the same
// number of entries, just shifted to post-values:
//
//	block  │ deprecated     │ new
//	───────┼────────────────┼──────
//	  B₁   │  0             │ N₁
//	  B₂   │  N₁            │ N₂
//	  B₃   │  N₂            │ N₃
//	───────┼────────────────┼──────
//	  > B₃ │  contract      │ N₃ (last entry — self-contained)
//	          .Nonce          ← deprecated must reach into the Contract
//	                            record for any block past the last change
//
// A nonzero nonce gets an entry at the floor with its value there: the first
// pre-value, or the head nonce when no history is left. Unpruned, it is always zero.
// Deprecated rows are wiped with the whole bucket once the phase completes.
func (i *nonceIngestor) Run(index int, addr felt.Address, outputs chan<- common.Task) error {
	addrFelt := (*felt.Felt)(&addr)

	curTask := &i.Tasks[index]
	deprecatedPrefix := db.DeprecatedContractNonceHistoryKey(addrFelt)

	depIt, err := i.Database.NewIterator(deprecatedPrefix, true)
	if err != nil {
		return fmt.Errorf("nonce: open deprecated iter(%s): %w", addrFelt, err)
	}
	defer depIt.Close()

	hasHistory := depIt.First()
	if !hasHistory {
		// No deprecated rows and new ones exist: a rerun after the phase completed.
		migrated, err := hasEntries(i.Database, db.ContractNonceHistoryKey(addrFelt))
		if err != nil || migrated {
			return err
		}
	}

	contract, err := state.GetContract(i.Database, addrFelt)
	if err != nil {
		return fmt.Errorf("nonce: GetContract(%s): %w", addrFelt, err)
	}

	floorNonce := contract.Nonce
	if hasHistory {
		rawValue, err := depIt.Value()
		if err != nil {
			return fmt.Errorf("nonce(%s): %w", addrFelt, err)
		}
		floorNonce = felt.FromBytes[felt.Felt](rawValue)
	}
	if !floorNonce.IsZero() {
		if err := i.write(curTask, outputs, addrFelt, i.floor, &floorNonce); err != nil {
			return err
		}
	}

	for hasHistory {
		block, err := parseBlockKey(depIt.Key(), deprecatedPrefix)
		if err != nil {
			return fmt.Errorf("nonce(%s): %w", addrFelt, err)
		}
		hasNext := depIt.Next()
		historyValue := contract.Nonce
		if hasNext {
			rawValue, err := depIt.Value()
			if err != nil {
				return fmt.Errorf("nonce(%s): %w", addrFelt, err)
			}
			historyValue = felt.FromBytes[felt.Felt](rawValue)
		}
		if err := i.write(curTask, outputs, addrFelt, block, &historyValue); err != nil {
			return err
		}
		hasHistory = hasNext
	}

	curTask.CompletedAddrs++
	return nil
}

func (i *nonceIngestor) write(
	t *common.Task, outputs chan<- common.Task, addr *felt.Felt, block uint64, nonce *felt.Felt,
) error {
	if err := state.WriteNonceHistory(t.Batch, addr, block, nonce); err != nil {
		return err
	}
	t.EntryCount++
	return i.Flush(t, outputs)
}
