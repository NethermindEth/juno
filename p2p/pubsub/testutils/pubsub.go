package testutils

import (
	"bytes"
	"crypto/ed25519"
	"sync"
	"testing"

	"github.com/NethermindEth/juno/consensus/p2p/config"
	"github.com/NethermindEth/juno/p2p/pubsub"
	"github.com/NethermindEth/juno/p2p/starknetp2p"
	"github.com/NethermindEth/juno/utils"
	libp2p "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
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
	wg := sync.WaitGroup{}
	for i := range nodes {
		wg.Go(func() {
			var err error
			nodes[i].Host, err = pubsub.GetHost(mockKey(i), hostAddress)
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, nodes[i].Host.Close())
			})
		})
	}
	wg.Wait()

	// This is a workaround to make sure that all `GetBootstrapPeers` are executed
	// concurrently once, which ensures that all nodes use the bootstrap peers.
	onces := make([]sync.Once, len(nodes))
	ready := make(chan struct{})
	bootstrapWg := sync.WaitGroup{}
	bootstrapWg.Add(len(nodes))
	go func() {
		bootstrapWg.Wait()
		close(ready)
	}()

	for i := range nodes {
		wg.Go(func() {
			peers := make([]peer.AddrInfo, 0, len(adjacentNodes[i]))
			for j := range adjacentNodes[i] {
				peers = append(peers, peer.AddrInfo{
					ID:    nodes[j].Host.ID(),
					Addrs: nodes[j].Host.Addrs(),
				})
			}
			nodes[i].GetBootstrapPeers = func() []peer.AddrInfo {
				onces[i].Do(func() {
					bootstrapWg.Done()
				})
				<-ready
				return peers
			}
		})
	}
	wg.Wait()

	return nodes
}

func (n Nodes) JoinTopic(
	t *testing.T,
	network *utils.Network,
	protocolID starknetp2p.Protocol,
	topicName string,
) []*libp2p.Topic {
	return iter.Map(n, func(node *Node) *libp2p.Topic {
		pubSub, closer, err := pubsub.Run(
			t.Context(),
			node.Host,
			network,
			protocolID,
			node.GetBootstrapPeers,
			config.DefaultBufferSizes.PubSubQueueSize,
		)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, closer())
		})

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
