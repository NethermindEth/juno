package history

import (
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/migration/state/newstate/internal/common"
)

type nonceIngestor struct {
	common.BaseIngestor
	scratches []historyScratch
}

var _ pipeline.State[felt.Address, common.Task] = (*nonceIngestor)(nil)

func newNonceIngestor(
	sem semaphore.ResourceSemaphore[db.Batch],
	database db.KeyValueReader,
) *nonceIngestor {
	return &nonceIngestor{
		BaseIngestor: common.NewBaseIngestor(sem, database),
		scratches:    make([]historyScratch, common.IngestorCount),
	}
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
// Contracts with no deprecated nonce history are skipped. Deprecated rows
// are deleted at the end of the run.
func (i *nonceIngestor) Run(index int, addr felt.Address, outputs chan<- common.Task) error {
	addrFelt := (*felt.Felt)(&addr)
	task := &i.Tasks[index]
	scratch := &i.scratches[index]

	prefix := fillAddressKey(scratch.deprecatedPrefix[:], db.DeprecatedContractNonceHistory, &addr)
	depIt, err := i.Database.NewIterator(prefix, true)
	if err != nil {
		return fmt.Errorf("opening deprecated nonce history of %s: %w", addrFelt.String(), err)
	}
	defer depIt.Close()
	if !depIt.First() {
		return nil
	}

	contractKey := fillAddressKey(scratch.contractKey[:], db.Contract, &addr)
	headNonce, err := readHeadNonce(i.Database, contractKey)
	if err != nil {
		return fmt.Errorf("reading contract record of %s: %w", addrFelt.String(), err)
	}

	// key trails one deprecated row behind the iterator: it names the block
	// whose post-value the row being read supplies.
	key := scratch.key[:blockKeyLen]
	if err := fillHistoryKeyFrom(key, db.ContractNonceHistory, depIt.UncopiedKey()); err != nil {
		return fmt.Errorf("%s: %w", addrFelt.String(), err)
	}

	for depIt.Next() {
		// This row's stored pre-value is the nonce in effect after the block
		// key names, and needs no decoding to move.
		value, err := depIt.UncopiedValue()
		if err != nil {
			return fmt.Errorf("reading deprecated nonce of %s: %w", addrFelt.String(), err)
		}
		if err := task.Batch.Put(key, value); err != nil {
			return err
		}

		if err := fillHistoryKeyFrom(key, db.ContractNonceHistory, depIt.UncopiedKey()); err != nil {
			return fmt.Errorf("%s: %w", addrFelt.String(), err)
		}
		task.EntryCount++
		if err := i.Flush(task, outputs); err != nil {
			return err
		}
	}

	// The last entry's value is the head nonce, which the history never stored.
	if err := task.Batch.Put(key, headNonce[:]); err != nil {
		return err
	}
	task.EntryCount++
	if err := i.Flush(task, outputs); err != nil {
		return err
	}

	task.CompletedAddrs++
	return nil
}
