package syncl1

import (
	"context"
	"fmt"
	"math/big"
	"runtime"

	"github.com/NethermindEth/juno/l1data"
	"github.com/sourcegraph/conc/stream"
)

type IteratorCallback func(encodedDiff []*big.Int, log *l1data.LogStateUpdate) error

type StateUpdateIterator struct {
	logFetcher  l1data.StateUpdateLogFetcher
	diffFetcher l1data.StateDiffFetcher
	callback    IteratorCallback
}

func NewIterator(logFetcher l1data.StateUpdateLogFetcher, diffFetcher l1data.StateDiffFetcher, cb IteratorCallback) *StateUpdateIterator {
	return &StateUpdateIterator{
		logFetcher:  logFetcher,
		diffFetcher: diffFetcher,
		callback:    cb,
	}
}

func (iter *StateUpdateIterator) Run(ctx context.Context, startHeight, maxHeight, startEthHeight uint64) error {
	streamSync := stream.New().WithMaxGoroutines(runtime.GOMAXPROCS(0) * 3)
	ctxSync, cancelSync := context.WithCancelCause(ctx)
	stateUpdateLogs := make([]*l1data.LogStateUpdate, 0)
	ethHeight := startEthHeight
	for nextStarknetBlockNumber := startHeight; nextStarknetBlockNumber <= maxHeight; {
		if err := ctxSync.Err(); err != nil {
			break
		}
		if len(stateUpdateLogs) == 0 {
			var err error
			stateUpdateLogs, err = iter.logFetcher.StateUpdateLogs(ctx, ethHeight, nextStarknetBlockNumber)
			if err != nil {
				cancelSync(err)
				break
			}
			// Set ethHeight to the last searched block.
			ethHeight = stateUpdateLogs[len(stateUpdateLogs)-1].Raw.BlockNumber
		}
		currentLog := *stateUpdateLogs[0]                //nolint:gosec
		currentCtx, currentCancel := ctxSync, cancelSync // TODO is this necessary?
		streamSync.Go(func() stream.Callback {
			encodedDiff, err := iter.diffFetcher.StateDiff(currentCtx, currentLog.Raw.TxIndex, currentLog.Raw.BlockNumber)
			if err != nil {
				currentCancel(err)
				return func() {}
			}
			return func() {
				if currentCtx.Err() != nil {
					return
				}
				if err := iter.callback(encodedDiff, &currentLog); err != nil {
					currentCancel(fmt.Errorf("callback: %v", err))
					return
				}
			}
		})
		stateUpdateLogs[0] = nil              //nolint:gosec // Avoid memory leak.
		stateUpdateLogs = stateUpdateLogs[1:] //nolint:gosec
		nextStarknetBlockNumber++
	}
	streamSync.Wait()
	cancelSync(nil)
	if err := context.Cause(ctxSync); err != nil && err != ctxSync.Err() {
		return err
	}
	return nil
}
