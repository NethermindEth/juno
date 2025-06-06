package sequencer

import (
	"context"

	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/utils"
)

// Execute a single block. Useful for tests.
func (s *Sequencer) RunOnce() error {
	if err := s.builder.ClearPending(); err != nil {
		s.log.Errorw("clearing pending", "err", err)
	}

	if err := s.builder.InitPendingBlock(s.sequencerAddress); err != nil {
		return err
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
		return err
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

	if err := s.builder.ClearPending(); err != nil {
		return err
	}
	if err := s.builder.InitPendingBlock(s.sequencerAddress); err != nil {
		return err
	}
	return nil
}
