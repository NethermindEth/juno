package history

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/migration/state/common"
)

type nonceIngestor struct {
	common.BaseIngestor
}

var _ pipeline.State[felt.Address, common.Task] = (*nonceIngestor)(nil)

func newNonceIngestor(
	ctx context.Context,
	sem semaphore.ResourceSemaphore[db.Batch],
	database db.KeyValueReader,
) *nonceIngestor {
	return &nonceIngestor{BaseIngestor: common.NewBaseIngestor(ctx, sem, database)}
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

	curTask := &i.Tasks[index]
	deprecatedPrefix := db.DeprecatedContractNonceHistoryKey(addrFelt)

	depIt, err := i.Database.NewIterator(deprecatedPrefix, true)
	if err != nil {
		return fmt.Errorf("nonce: open deprecated iter(%s): %w", addrFelt, err)
	}
	defer depIt.Close()
	if !depIt.First() {
		return nil
	}

	contract, err := state.GetContract(i.Database, addrFelt)
	if err != nil {
		return fmt.Errorf("nonce: GetContract(%s): %w", addrFelt, err)
	}

	for {
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
		err = state.WriteNonceHistory(curTask.Batch, addrFelt, block, &historyValue)
		if err != nil {
			return err
		}
		curTask.EntryCount++
		if err := i.Flush(curTask, outputs); err != nil {
			return err
		}
		if !hasNext {
			break
		}
	}

	err = curTask.Batch.DeleteRange(deprecatedPrefix, dbutils.UpperBound(deprecatedPrefix))
	if err != nil {
		return fmt.Errorf("nonce: DeleteRange deprecated(%s): %w", addrFelt, err)
	}

	curTask.CompletedAddrs++
	return nil
}
