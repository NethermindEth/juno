package statehistory

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
)

type nonceIngestor struct {
	baseIngestor
}

var _ pipeline.State[*felt.Felt, task] = (*nonceIngestor)(nil)

func newNonceIngestor(
	ctx context.Context,
	sem semaphore.ResourceSemaphore[db.Batch],
	database db.KeyValueReader,
) *nonceIngestor {
	return &nonceIngestor{baseIngestor: newBaseIngestor(ctx, sem, database)}
}

func (i *nonceIngestor) Run(index int, addr *felt.Felt, outputs chan<- task) error {
	t := &i.tasks[index]
	deprecatedPrefix := db.DeprecatedContractNonceHistoryKey(addr)

	depIt, err := i.database.NewIterator(deprecatedPrefix, true)
	if err != nil {
		return fmt.Errorf("nonce: open deprecated iter(%s): %w", addr, err)
	}
	defer depIt.Close()
	if !depIt.First() {
		return nil
	}

	contract, err := state.GetContract(i.database, addr)
	if err != nil {
		return fmt.Errorf("nonce: GetContract(%s): %w", addr, err)
	}

	for {
		block, err := parseBlockKey(depIt.Key(), deprecatedPrefix)
		if err != nil {
			return fmt.Errorf("nonce(%s): %w", addr, err)
		}
		hasNext := depIt.Next()
		historyValue := contract.Nonce
		if hasNext {
			rawValue, err := depIt.Value()
			if err != nil {
				return fmt.Errorf("nonce(%s): %w", addr, err)
			}
			historyValue = felt.FromBytes[felt.Felt](rawValue)
		}
		err = state.WriteNonceHistory(t.batch, addr, block, &historyValue)
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
		return fmt.Errorf("nonce: DeleteRange deprecated(%s): %w", addr, err)
	}
	t.completedAddrs++
	return nil
}
