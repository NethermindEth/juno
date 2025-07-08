package sequencer

import (
	"context"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/utils"
)

// Execute a single block. Useful for tests.
func (s *Sequencer) RunOnce() (*core.Header, error) {
	err := s.buildState.ClearPending()
	if err != nil {
		s.log.Errorw("clearing pending", "err", err)
	}

	if err := s.initPendingBlock(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if pErr := s.listenPool(ctx); pErr != nil {
		if pErr != mempool.ErrTxnPoolEmpty {
			s.log.Warnw("listening pool", "err", pErr)
		}
	}

	pending, err := s.Pending()
	if err != nil {
		s.log.Infof("Failed to get pending block")
	}
	if err := s.builder.Finalise(pending, utils.Sign(s.privKey), s.privKey); err != nil {
		return nil, err
	}
	s.log.Infof("Finalised new block")
	if s.plugin != nil {
		err := s.plugin.NewBlock(pending.Block, pending.StateUpdate, pending.NewClasses)
		if err != nil {
			s.log.Errorw("error sending new block to plugin", err)
		}
	}
	// push the new head to the feed
	s.subNewHeads.Send(pending.Block)

	if err := s.initPendingBlock(); err != nil {
		return nil, err
	}
	return pending.Block.Header, nil
}
