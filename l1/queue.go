package l1

import (
	"github.com/NethermindEth/juno/l1/contract"
)

// queue is a priority queue that implements a "confirmation window."
// Given an L1 height, all [contract.LogStateUpdate]s emitted at least
// confirmationPeriod blocks before the height are "confirmed".
//
// Its abnormal API reflects the fact that it is only meant to be used
// by the L1 verifier.
type queue struct {
	confirmationPeriod uint64
	q                  map[uint64]*contract.StarknetLogStateUpdate
}

func newQueue(confirmationPeriod uint64) *queue {
	return &queue{
		confirmationPeriod: confirmationPeriod,
		q:                  make(map[uint64]*contract.StarknetLogStateUpdate),
	}
}

func (q *queue) Enqueue(update *contract.StarknetLogStateUpdate) {
	q.q[update.Raw.BlockNumber] = update
}

func (q *queue) Remove(l1BlockNumber uint64) {
	delete(q.q, l1BlockNumber)
}

func (q *queue) Reorg(forkedBlockNumber uint64) {
	for l1BlockNumber := range q.q {
		if l1BlockNumber >= forkedBlockNumber {
			q.Remove(l1BlockNumber)
		}
	}
}

// MaxConfirmed returns the most recent confirmed update and deletes
// all other confirmed updates.
func (q *queue) MaxConfirmed(l1Height uint64) *contract.StarknetLogStateUpdate {
	var maxConfirmed *contract.StarknetLogStateUpdate
	for l1BlockNumber, possibleHead := range q.q {
		if l1Height-l1BlockNumber >= q.confirmationPeriod {
			// We are careful to use >= here. If the max block is zero,
			// this will still work. Mostly helpful for tests.
			if maxConfirmed == nil || l1BlockNumber >= maxConfirmed.Raw.BlockNumber {
				maxConfirmed = possibleHead
			} else {
				q.Remove(l1BlockNumber)
			}
		}
	}
	return maxConfirmed
}
