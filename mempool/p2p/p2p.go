package p2p

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/consensus/p2p/buffered"
	"github.com/NethermindEth/juno/consensus/p2p/config"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/p2p/pubsub"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sourcegraph/conc"
)

const (
	chainID              = "1" // TODO: make this configurable
	mempoolProtocolID    = "mempool"
	transactionTopicName = "mempool_transaction_propagation"
)

type P2P struct {
	host             host.Host
	log              utils.Logger
	pool             mempool.Pool
	broadcaster      transactionBroadcaster
	listener         buffered.TopicSubscription
	config           *config.BufferSizes
	bootstrapPeersFn func() []peer.AddrInfo
}

func New(
	host host.Host,
	log utils.Logger,
	pool mempool.Pool,
	config *config.BufferSizes,
	bootstrapPeersFn func() []peer.AddrInfo,
) *P2P {
	return &P2P{
		host:             host,
		log:              log,
		pool:             pool,
		broadcaster:      NewTransactionBroadcaster(log, config.MempoolBroadcaster, config.RetryInterval),
		listener:         NewTransactionListener(log, pool, config.MempoolListener),
		config:           config,
		bootstrapPeersFn: bootstrapPeersFn,
	}
}

func (p *P2P) Run(ctx context.Context) error {
	gossipSub, err := pubsub.Run(
		ctx,
		chainID,
		mempoolProtocolID,
		p.host,
		p.config.PubSubQueueSize,
		p.bootstrapPeersFn,
	)
	if err != nil {
		return fmt.Errorf("unable to create gossipsub with error: %w", err)
	}

	topic, relayCancel, err := pubsub.JoinTopic(gossipSub, transactionTopicName)
	if err != nil {
		return fmt.Errorf("unable to join topic %s with error: %w", transactionTopicName, err)
	}
	defer relayCancel()
	defer topic.Close()

	wg := conc.NewWaitGroup()

	// Start broadcaster and listener loops
	wg.Go(func() {
		p.broadcaster.Loop(ctx, topic)
	})
	wg.Go(func() {
		p.listener.Loop(ctx, topic)
	})

	wg.Wait()
	return nil
}

func (p *P2P) Push(ctx context.Context, transaction *mempool.BroadcastedTransaction) error {
	if err := p.pool.Push(ctx, transaction); err != nil {
		return err
	}
	p.broadcaster.Broadcast(ctx, transaction)
	return nil
}
