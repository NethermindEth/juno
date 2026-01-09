package blocktransactions

import (
	"github.com/NethermindEth/juno/db"
)

// batchProvider provides batches to the ingestor and committer, rate limited using a semaphore.
type batchProvider struct {
	database db.KeyValueStore
	sem      chan struct{}
}

func newBatchProvider(database db.KeyValueStore, concurrency int) *batchProvider {
	sem := make(chan struct{}, concurrency)
	for range concurrency {
		sem <- struct{}{}
	}
	return &batchProvider{
		database: database,
		sem:      sem,
	}
}

func (b *batchProvider) get() task {
	<-b.sem
	return task{
		batch:           b.database.NewBatchWithSize(batchByteSize),
		totalTxCount:    0,
		totalBlockCount: 0,
	}
}

func (b *batchProvider) put() {
	b.sem <- struct{}{}
}

func (b *batchProvider) close() {
	close(b.sem)
}
