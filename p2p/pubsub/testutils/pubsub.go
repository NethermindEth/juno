package testutils

import (
	"bytes"
	"crypto/ed25519"
	"testing"

	"github.com/NethermindEth/juno/consensus/p2p/config"
	"github.com/NethermindEth/juno/p2p/pubsub"
	libp2p "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/conc/iter"
	"github.com/stretchr/testify/require"
)

const hostAddress = "/ip4/127.0.0.1/tcp/0"

type Node struct {
	Host              host.Host
	GetBootstrapPeers func() []peer.AddrInfo
}

type Nodes []Node

func BuildNetworks(
	t *testing.T,
	adjacentNodes AdjacentNodes,
) Nodes {
	nodes := make([]Node, len(adjacentNodes))
	wg := conc.NewWaitGroup()
	for i := range nodes {
		wg.Go(func() {
			var err error
			nodes[i].Host, err = pubsub.GetHost(mockKey(i), hostAddress)
			require.NoError(t, err)
		})
	}
	wg.Wait()

	wg = conc.NewWaitGroup()
	for i := range nodes {
		wg.Go(func() {
			peers := make([]peer.AddrInfo, 0)

			for j := range adjacentNodes[i] {
				other := peer.AddrInfo{
					ID:    nodes[j].Host.ID(),
					Addrs: nodes[j].Host.Addrs(),
				}
				peers = append(peers, other)
			}

			nodes[i].GetBootstrapPeers = func() []peer.AddrInfo {
				return peers
			}
		})
	}
	wg.Wait()

	return nodes
}

func (n Nodes) JoinTopic(
	t *testing.T,
	chainID, protocolID protocol.ID,
	topicName string,
) []*libp2p.Topic {
	return iter.Map(n, func(node *Node) *libp2p.Topic {
		pubSub, err := pubsub.Run(
			t.Context(),
			chainID,
			protocolID,
			node.Host,
			config.DefaultBufferSizes.PubSubQueueSize,
			node.GetBootstrapPeers,
		)
		require.NoError(t, err)

		topic, relayCancel, err := pubsub.JoinTopic(pubSub, topicName)
		require.NoError(t, err)
		t.Cleanup(relayCancel)

		return topic
	})
}

func mockKey(nodeIndex int) crypto.PrivKey {
	seed := make([]byte, ed25519.SeedSize)
	seed[0] = byte(nodeIndex)
	reader := bytes.NewReader(seed)
	privKey, _, err := crypto.GenerateEd25519Key(reader)
	if err != nil {
		panic(err)
	}
	return privKey
}
