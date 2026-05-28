package headstate

import (
	"context"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/migration/state/common"
)

type ingestor struct {
	common.BaseIngestor
}

var _ pipeline.State[felt.Address, common.Task] = (*ingestor)(nil)

func newIngestor(
	ctx context.Context,
	sem semaphore.ResourceSemaphore[db.Batch],
	database db.KeyValueReader,
) *ingestor {
	return &ingestor{BaseIngestor: common.NewBaseIngestor(ctx, sem, database)}
}

func (c *ingestor) Run(index int, addr felt.Address, outputs chan<- common.Task) error {
	curTask := &c.Tasks[index]

	sizeBefore := curTask.Batch.Size()
	if err := c.ingestAddress(curTask.Batch, &addr); err != nil {
		return err
	}
	if curTask.Batch.Size() > sizeBefore {
		curTask.CompletedAddrs++
	}

	return c.Flush(curTask, outputs)
}

func (c *ingestor) ingestAddress(batch db.Batch, addr *felt.Address) error {
	addrFelt := (*felt.Felt)(addr)

	already, err := state.HasContract(c.Database, addrFelt)
	if err != nil {
		return fmt.Errorf("HasContract(%s): %w", addr, err)
	}
	if already {
		return nil
	}

	classHash, err := core.GetContractClassHash(c.Database, addrFelt)
	if err != nil {
		return fmt.Errorf("GetContractClassHash(%s): %w", addr, err)
	}

	nonce, err := core.GetContractNonce(c.Database, addrFelt)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return fmt.Errorf("GetContractNonce(%s): %w", addr, err)
		}
		nonce = felt.Zero
	}

	height, err := core.GetContractDeploymentHeight(c.Database, addrFelt)
	if err != nil {
		return fmt.Errorf("GetContractDeploymentHeight(%s): %w", addr, err)
	}

	if err := state.WriteContract(batch, addrFelt, nonce, classHash, height); err != nil {
		return fmt.Errorf("WriteContract(%s): %w", addr, err)
	}
	return nil
}
