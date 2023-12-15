package l1data

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

//go:generate mockgen -destination=../mocks/mock_fetcher.go -package=mocks github.com/NethermindEth/juno/l1data StateDiffFetcher
type StateDiffFetcher interface {
	StateDiff(ctx context.Context, logStateUpdateTxIndex uint, logStateUpdateBlockNumber uint64) ([]*big.Int, error)
}

type Fetcher struct {
	l1data L1Data
}

func NewStateDiffFetcher(l1data L1Data) Fetcher {
	return Fetcher{
		l1data: l1data,
	}
}

func (f Fetcher) StateDiff(ctx context.Context, logStateUpdateTxIndex uint, logStateUpdateBlockNumber uint64) ([]*big.Int, error) {
	fact, err := f.l1data.StateTransitionFact(ctx, logStateUpdateTxIndex, logStateUpdateBlockNumber)
	if err != nil {
		return nil, fmt.Errorf("get state transition fact: %w", err)
	}
	mempageHashesLog, err := f.l1data.MemoryPagesHashesLog(ctx, fact, logStateUpdateBlockNumber)
	if err != nil {
		return nil, fmt.Errorf("get log memory pages hashes log: %w", err)
	}
	continuousLogs, err := f.l1data.MemoryPageFactContinuousLogs(ctx, mempageHashesLog.PagesHashes, mempageHashesLog.Raw.BlockNumber)
	if err != nil {
		return nil, fmt.Errorf("get memory page fact continuous logs: %w", err)
	}
	txHashes := make([]common.Hash, len(continuousLogs))
	for i, log := range continuousLogs {
		txHashes[i] = log.Raw.TxHash
	}
	encodedStateDiff, err := f.l1data.EncodedStateDiff(ctx, txHashes)
	if err != nil {
		return nil, fmt.Errorf("get encoded state diff: %w", err)
	}
	return encodedStateDiff, nil
}
