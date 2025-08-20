package pubsub

import (
	"context"
	"fmt"
	"strings"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
)

const (
	protocolPrefix   = "starknet"
	gossipSubHistory = 60
)

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
	chainID protocol.ID,
	protocolID protocol.ID,
	host host.Host,
	pubSubQueueSize int,
	bootstrapPeersFn func() []peer.AddrInfo,
) (*pubsub.PubSub, error) {
	dht, err := dht.New(
		ctx,
		host,
		dht.ProtocolPrefix("/"+protocolPrefix),
		dht.ProtocolExtension("/"+chainID),
		dht.ProtocolExtension("/"+protocolID),
		dht.BootstrapPeersFunc(bootstrapPeersFn),
		dht.Mode(dht.ModeServer),
	)
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

func ExtractPeers(peers string) ([]peer.AddrInfo, error) {
	if peers == "" {
		return nil, nil
	}

	peerAddrs := []peer.AddrInfo{}
	for peerStr := range strings.SplitSeq(peers, ",") {
		peerAddr, err := peer.AddrInfoFromString(peerStr)
		if err != nil {
			return nil, fmt.Errorf("unable to parse peer address %q: %w", peerStr, err)
		}
		peerAddrs = append(peerAddrs, *peerAddr)
	}
	return peerAddrs, nil
}
