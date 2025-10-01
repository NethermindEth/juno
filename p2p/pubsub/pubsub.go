package pubsub

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/p2p/dht"
	"github.com/NethermindEth/juno/p2p/starknetp2p"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
)

const gossipSubHistory = 60

func GetHost(hostPrivateKey crypto.PrivKey, hostAddress string) (host.Host, error) {
	return libp2p.New(
		libp2p.ListenAddrStrings(hostAddress),
		libp2p.Identity(hostPrivateKey),
		// libp2p.UserAgent(makeAgentName(version)),
		// // Use address factory to add the public address to the list of
		// // addresses that the node will advertise.
		// libp2p.AddrsFactory(addressFactory),
		// If we know the public ip, enable the relay service.
		libp2p.EnableRelayService(),
		// When listening behind NAT, enable peers to try to poke thought the
		// NAT in order to reach the node.
		libp2p.EnableHolePunching(),
		// Try to open a port in the NAT router to accept incoming connections.
		libp2p.NATPortMap(),
	)
}

func Run(
	ctx context.Context,
	host host.Host,
	network *utils.Network,
	starknetProtocol starknetp2p.Protocol,
	bootstrapPeersFn func() []peer.AddrInfo,
	pubSubQueueSize int,
) (*pubsub.PubSub, error) {
	dht, err := dht.New(ctx, host, network, starknetProtocol, bootstrapPeersFn)
	if err != nil {
		return nil, fmt.Errorf("unable to create dht with error: %w", err)
	}

	params := pubsub.DefaultGossipSubParams()
	params.HistoryLength = gossipSubHistory
	params.HistoryGossip = gossipSubHistory

	return pubsub.NewGossipSub(
		ctx,
		host,
		pubsub.WithPeerOutboundQueueSize(pubSubQueueSize),
		pubsub.WithValidateQueueSize(pubSubQueueSize),
		pubsub.WithDiscovery(routing.NewRoutingDiscovery(dht)),
		pubsub.WithGossipSubParams(params),
	)
}

func JoinTopic(pubSub *pubsub.PubSub, topicName string) (*pubsub.Topic, func(), error) {
	topic, err := pubSub.Join(topicName)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to join topic %s with error: %w", topicName, err)
	}

	// Make sure that the host starts connecting to other nodes
	relayCancel, err := topic.Relay()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to relay topic %s with error: %w", topicName, err)
	}

	return topic, relayCancel, nil
}
