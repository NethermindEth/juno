package sync

import (
	"context"

	"github.com/NethermindEth/juno/sync/preconfirmed"
)

// pollPendingData launches the pre_confirmed chain poller.
func (s *Synchronizer) pollPendingData(ctx context.Context) {
	if s.preConfirmedPollInterval == 0 {
		s.logger.Info("Pre-confirmed block polling is disabled")
		return
	}

	poller := preconfirmed.NewPoller(
		s.dataSource,
		s.preConfirmed,
		s.blockchain,
		s.preConfirmedDataFeed,
		&s.highestBlockHeader,
		s.preConfirmedPollInterval,
		s.logger,
	)
	poller.Run(ctx)
}
