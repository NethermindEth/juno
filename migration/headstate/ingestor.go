package headstate

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
)

type ingestor struct {
	database       db.KeyValueReader
	batchSemaphore semaphore.ResourceSemaphore[db.Batch]
	tasks          []task
}

func newIngestor(
	database db.KeyValueReader,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
) *ingestor {
	tasks := make([]task, ingestorCount)
	for i := range tasks {
		tasks[i] = task{batch: batchSemaphore.GetBlocking()}
	}
	return &ingestor{
		database:       database,
		batchSemaphore: batchSemaphore,
		tasks:          tasks,
	}
}

var _ pipeline.State[felt.Address, task] = (*ingestor)(nil)

func (c *ingestor) Run(index int, addr felt.Address, outputs chan<- task) error {
	t := &c.tasks[index]

	sizeBefore := t.batch.Size()
	if err := c.ingestAddress(t.batch, addr); err != nil {
		return err
	}
	if t.batch.Size() > sizeBefore {
		t.addrCount++
	}

	if t.batch.Size() >= targetBatchByteSize {
		outputs <- task{batch: t.batch, addrCount: t.addrCount}
		t.addrCount = 0
		t.batch = c.batchSemaphore.GetBlocking()
	}
	return nil
}

func (c *ingestor) Done(index int, outputs chan<- task) error {
	outputs <- c.tasks[index]
	return nil
}

func (c *ingestor) ingestAddress(batch db.Batch, addr felt.Address) error {
	addrFelt := felt.Felt(addr)

	already, err := state.HasContract(c.database, &addrFelt)
	if err != nil {
		return fmt.Errorf("HasContract(%s): %w", &addrFelt, err)
	}
	if already {
		return nil
	}

	classHash, err := core.GetContractClassHash(c.database, &addrFelt)
	if err != nil {
		return fmt.Errorf("GetContractClassHash(%s): %w", &addrFelt, err)
	}

	nonce, err := core.GetContractNonce(c.database, &addrFelt)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return fmt.Errorf("GetContractNonce(%s): %w", &addrFelt, err)
		}
		nonce = felt.Zero
	}

	height, err := core.GetContractDeploymentHeight(c.database, &addrFelt)
	if err != nil {
		return fmt.Errorf("GetContractDeploymentHeight(%s): %w", &addrFelt, err)
	}

	if err := state.WriteContract(batch, &addrFelt, nonce, classHash, height); err != nil {
		return fmt.Errorf("WriteContract(%s): %w", &addrFelt, err)
	}
	return nil
}
