package p2p

import (
	"context"
	"time"

	"github.com/NethermindEth/juno/adapters/mempool2p2p"
	"github.com/NethermindEth/juno/consensus/p2p/buffered"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/mempool/transaction"
	"go.uber.org/zap"
)

type transactionBroadcaster struct {
	buffered.ProtoBroadcaster[*transaction.MempoolTransaction]
	logger log.Logger
}

func NewTransactionBroadcaster(
	logger log.Logger,
	bufferSize int,
	retryInterval time.Duration,
) transactionBroadcaster {
	return transactionBroadcaster{
		logger: logger,
		ProtoBroadcaster: buffered.NewProtoBroadcaster[*transaction.MempoolTransaction](
			logger,
			bufferSize,
			retryInterval,
			nil,
		),
	}
}

func (b *transactionBroadcaster) Broadcast(ctx context.Context, message *mempool.BroadcastedTransaction) {
	msg, err := mempool2p2p.AdaptTransaction(message)
	if err != nil {
		b.logger.Error("unable to convert transaction", zap.Error(err))
		return
	}

	b.ProtoBroadcaster.Broadcast(ctx, msg)
}
