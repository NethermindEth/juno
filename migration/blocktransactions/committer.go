package blocktransactions

import (
	"github.com/NethermindEth/juno/utils"
)

type committer struct {
	counter       counter
	logger        utils.SimpleLogger
	batchProvider *batchProvider
}

var _ state[task, struct{}] = (*committer)(nil)

func newCommitter(logger utils.SimpleLogger, batchProvider *batchProvider) *committer {
	return &committer{
		logger:        logger,
		counter:       newCounter(logger, timeLogRate),
		batchProvider: batchProvider,
	}
}

func (c *committer) run(_ int, task task, _ chan<- struct{}) error {
	c.logger.Debugw(
		"writing batch",
		"txCount", task.totalTxCount,
		"blockCount", task.totalBlockCount,
		"batchSize", task.batch.Size(),
	)
	defer c.logger.Debugw(
		"wrote batch",
		"txCount", task.totalTxCount,
		"blockCount", task.totalBlockCount,
		"batchSize", task.batch.Size(),
	)

	byteSize := uint64(task.batch.Size())
	if err := task.batch.Write(); err != nil {
		return err
	}

	c.counter.log(byteSize, task.totalTxCount, task.totalBlockCount)
	c.batchProvider.put()
	return nil
}

func (c *committer) done(int, chan<- struct{}) error {
	return nil
}
