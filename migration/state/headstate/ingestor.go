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

type task struct {
	batch          db.Batch
	completedAddrs int
}

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
	curTask := &c.tasks[index]

	sizeBefore := curTask.batch.Size()
	if err := c.ingestAddress(curTask.batch, &addr); err != nil {
		return err
	}
	if curTask.batch.Size() > sizeBefore {
		curTask.completedAddrs++
	}

	if curTask.batch.Size() >= targetBatchByteSize {
		outputs <- task{batch: curTask.batch, completedAddrs: curTask.completedAddrs}
		curTask.completedAddrs = 0
		curTask.batch = c.batchSemaphore.GetBlocking()
	}
	return nil
}

func (c *ingestor) Done(index int, outputs chan<- task) error {
	outputs <- c.tasks[index]
	return nil
}

func (c *ingestor) ingestAddress(batch db.Batch, addr *felt.Address) error {
	addrFelt := (*felt.Felt)(addr)

	already, err := state.HasContract(c.database, addrFelt)
	if err != nil {
		return fmt.Errorf("HasContract(%s): %w", addr, err)
	}
	if already {
		return nil
	}

	classHash, err := core.GetContractClassHash(c.database, addrFelt)
	if err != nil {
		return fmt.Errorf("GetContractClassHash(%s): %w", addr, err)
	}

	nonce, err := core.GetContractNonce(c.database, addrFelt)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return fmt.Errorf("GetContractNonce(%s): %w", addr, err)
		}
		nonce = felt.Zero
	}

	height, err := core.GetContractDeploymentHeight(c.database, addrFelt)
	if err != nil {
		return fmt.Errorf("GetContractDeploymentHeight(%s): %w", addr, err)
	}

	if err := state.WriteContract(batch, addrFelt, nonce, classHash, height); err != nil {
		return fmt.Errorf("WriteContract(%s): %w", addr, err)
	}
	return nil
}
