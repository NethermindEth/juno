package sync

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/utils/pipeline"
)

// Todo: refactor the sync pkg to re-use this logic. Certain code
// has been duplicated to keep the git-diff smaller.

func (s *Service) SyncToChannel(ctx context.Context, blockCh chan<- BlockBody) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.client = NewClient(s.randomPeerStream, s.network, s.log)

	for i := 0; ; i++ {
		if err := ctx.Err(); err != nil {
			break
		}
		s.log.Debugw("Continuous iteration", "i", i)

		iterCtx, cancelIteration := context.WithCancel(ctx)
		nextHeight, err := s.getNextHeight()
		if err != nil {
			s.logError("Failed to get current height", err)
			cancelIteration()
			continue
		}

		s.log.Infow("Start Pipeline", "Current height", nextHeight-1, "Start", nextHeight)

		// todo change iteration to fetch several objects uint64(min(blockBehind, maxBlocks))
		err = s.processBlockToChannel(iterCtx, blockCh, uint64(nextHeight))
		if err != nil {
			s.logError("Failed to get block", err)
			cancelIteration()
			continue
		}
		cancelIteration()
	}
}

func (s *Service) processBlockToChannel(ctx context.Context, blockCh chan<- BlockBody, blockNumber uint64) error {
	headersAndSigsCh, err := s.genHeadersAndSigs(ctx, blockNumber)
	if err != nil {
		return fmt.Errorf("failed to get block headers parts: %w", err)
	}

	txsCh, err := s.genTransactions(ctx, blockNumber)
	if err != nil {
		return fmt.Errorf("failed to get transactions: %w", err)
	}

	eventsCh, err := s.genEvents(ctx, blockNumber)
	if err != nil {
		return fmt.Errorf("failed to get events: %w", err)
	}

	classesCh, err := s.genClasses(ctx, blockNumber)
	if err != nil {
		return fmt.Errorf("failed to get classes: %w", err)
	}

	stateDiffsCh, err := s.genStateDiffs(ctx, blockNumber)
	if err != nil {
		return fmt.Errorf("failed to get state diffs: %w", err)
	}

	blocksCh := pipeline.Bridge(ctx, s.processSpecBlockParts(ctx, blockNumber, pipeline.FanIn(ctx,
		pipeline.Stage(ctx, headersAndSigsCh, specBlockPartsFunc[specBlockHeaderAndSigs]),
		pipeline.Stage(ctx, classesCh, specBlockPartsFunc[specClasses]),
		pipeline.Stage(ctx, stateDiffsCh, specBlockPartsFunc[specContractDiffs]),
		pipeline.Stage(ctx, txsCh, specBlockPartsFunc[specTxWithReceipts]),
		pipeline.Stage(ctx, eventsCh, specBlockPartsFunc[specEvents]),
	)))
	for b := range blocksCh {
		blockCh <- b
	}
	return nil
}
