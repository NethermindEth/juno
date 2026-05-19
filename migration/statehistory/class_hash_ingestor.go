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

type classHashIngestor struct {
	baseIngestor
}

var _ pipeline.State[*felt.Felt, task] = (*classHashIngestor)(nil)

func newClassHashIngestor(
	ctx context.Context,
	sem semaphore.ResourceSemaphore[db.Batch],
	database db.KeyValueReader,
) *classHashIngestor {
	return &classHashIngestor{baseIngestor: newBaseIngestor(ctx, sem, database)}
}

func (i *classHashIngestor) Run(index int, addr *felt.Felt, outputs chan<- task) error {
	t := &i.tasks[index]

	deprecatedPrefix := db.DeprecatedContractClassHashHistoryKey(addr)
	contract, err := state.GetContract(i.database, addr)
	if err != nil {
		return fmt.Errorf("class-hash: GetContract(%s): %w", addr, err)
	}

	deployKey := db.ContractClassHashHistoryAtBlockKey(addr, contract.DeployedHeight)
	deployEntryExists, err := i.database.Has(deployKey)
	if err != nil {
		return fmt.Errorf("class-hash: Has(deploy entry): %w", err)
	}

	depIt, err := i.database.NewIterator(deprecatedPrefix, true)
	if err != nil {
		return fmt.Errorf("class-hash: open deprecated iter(%s): %w", addr, err)
	}
	defer depIt.Close()

	if !depIt.First() {
		if deployEntryExists {
			return nil
		}
		err = state.WriteClassHashHistory(
			t.batch,
			addr,
			contract.DeployedHeight,
			&contract.ClassHash,
		)
		if err != nil {
			return err
		}
		t.completedAddrs++
		t.entryCount++
		return i.flush(t, outputs)
	}

	rawValue, err := depIt.Value()
	if err != nil {
		return fmt.Errorf("class-hash: read first value(%s): %w", addr, err)
	}
	deployClassHash := felt.FromBytes[felt.Felt](rawValue)
	if err := state.WriteClassHashHistory(
		t.batch,
		addr,
		contract.DeployedHeight,
		&deployClassHash,
	); err != nil {
		return err
	}
	t.entryCount++
	if err := i.flush(t, outputs); err != nil {
		return err
	}

	// Shift-up loop: each block in the deprecated history gets the *next*
	// entry's value (since in the old layout the value at block B was the
	// value before B's write). The final block gets the head class hash.
	for {
		block, err := parseBlockKey(depIt.Key(), deprecatedPrefix)
		if err != nil {
			return fmt.Errorf("class-hash(%s): %w", addr, err)
		}
		hasNext := depIt.Next()
		historyValue := contract.ClassHash
		if hasNext {
			rawValue, err := depIt.Value()
			if err != nil {
				return fmt.Errorf("class-hash(%s): %w", addr, err)
			}
			historyValue = felt.FromBytes[felt.Felt](rawValue)
		}
		if err := state.WriteClassHashHistory(t.batch, addr, block, &historyValue); err != nil {
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
		return fmt.Errorf("class-hash: DeleteRange deprecated(%s): %w", addr, err)
	}
	t.completedAddrs++
	return nil
}
