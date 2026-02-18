package p2p

import (
	"context"

	"github.com/NethermindEth/juno/adapters/p2p2mempool"
	"github.com/NethermindEth/juno/consensus/p2p/buffered"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	mempooltransaction "github.com/starknet-io/starknet-p2pspecs/p2p/proto/mempool/transaction"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

func NewTransactionListener(
	network *utils.Network,
	log utils.Logger,
	pool mempool.Pool,
	bufferSize int,
	compiler compiler.Compiler,
) buffered.TopicSubscription {
	onMessage := func(ctx context.Context, msg *pubsub.Message) {
		var p2pTransaction mempooltransaction.MempoolTransaction
		if err := proto.Unmarshal(msg.Data, &p2pTransaction); err != nil {
			log.Error("unable to unmarshal transaction message", zap.Error(err))
			return
		}

		transaction, err := p2p2mempool.AdaptTransaction(
			ctx, compiler, &p2pTransaction, network,
		)
		if err != nil {
			log.Error("unable to convert transaction message to transaction", zap.Error(err))
			return
		}

		if err := pool.Push(ctx, &transaction); err != nil {
			log.Error("unable to push transaction to mempool", zap.Error(err))
		}
	}

	return buffered.NewTopicSubscription(log, bufferSize, onMessage)
}
