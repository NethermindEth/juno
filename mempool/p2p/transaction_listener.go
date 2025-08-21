package p2p

import (
	"context"

	"github.com/NethermindEth/juno/adapters/p2p2mempool"
	"github.com/NethermindEth/juno/consensus/p2p/buffered"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/utils"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	mempooltransaction "github.com/starknet-io/starknet-p2pspecs/p2p/proto/mempool/transaction"
	"google.golang.org/protobuf/proto"
)

func NewTransactionListener(
	log utils.Logger,
	pool mempool.Pool,
	bufferSize int,
) buffered.TopicSubscription {
	onMessage := func(ctx context.Context, msg *pubsub.Message) {
		var p2pTransaction mempooltransaction.MempoolTransaction
		if err := proto.Unmarshal(msg.Data, &p2pTransaction); err != nil {
			log.Errorw("unable to unmarshal transaction message", "error", err)
			return
		}

		transaction, err := p2p2mempool.AdaptTransaction(&p2pTransaction)
		if err != nil {
			log.Errorw("unable to convert transaction message to transaction", "error", err)
			return
		}

		if err := pool.Push(ctx, &transaction); err != nil {
			log.Errorw("unable to push transaction to mempool", "error", err)
		}
	}

	return buffered.NewTopicSubscription(log, bufferSize, onMessage)
}
