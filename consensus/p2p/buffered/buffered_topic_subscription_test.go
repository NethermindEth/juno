package buffered_test

import (
	"context"
	"fmt"
	"maps"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus/p2p/buffered"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/conc/iter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

const (
	topicName           = "test-buffered-topic-subscription"
	discoveryServiceTag = "test-buffered-topic-subscription-discovery"
	nodeCount           = 20
	messageCount        = 100
	throttledRate       = 3 * nodeCount * time.Millisecond
	logLevel            = zapcore.ErrorLevel
)

type node struct {
	host     host.Host
	topic    *pubsub.Topic
	messages []string
}

func TestBufferedTopicSubscription(t *testing.T) {
	t.Run(fmt.Sprintf("%d nodes, each sending %d messages", nodeCount, messageCount), func(t *testing.T) {
		logger, err := utils.NewZapLogger(utils.NewLogLevel(logLevel), true)
		require.NoError(t, err)

		var lastHost host.Host
		nodes := make([]node, nodeCount)

		for i := range nodes {
			nodes[i] = getNode(t)

			// Connect with the previous node if not the first node. The connection direction is random.
			if i > 0 {
				if rand.Intn(2) == 0 {
					connect(t, nodes[i].host, lastHost)
				} else {
					connect(t, lastHost, nodes[i].host)
				}
			}

			lastHost = nodes[i].host
		}

		allMessages := make(map[string]struct{})

		for i := range nodes {
			nodes[i].messages = make([]string, messageCount)
			for j := range nodes[i].messages {
				msg := fmt.Sprintf("message %d", i*messageCount+j)
				nodes[i].messages[j] = msg
				allMessages[msg] = struct{}{}
			}
		}

		iterator := iter.Iterator[node]{MaxGoroutines: len(nodes)}
		wg := sync.WaitGroup{}
		wg.Add(len(nodes))

		go func() {
			iterator.ForEachIdx(nodes, func(i int, destination *node) {
				logger := &utils.ZapLogger{SugaredLogger: logger.Named(fmt.Sprintf("destination-%d", i))}
				pending := maps.Clone(allMessages)
				subscription := buffered.NewTopicSubscription(logger, nodeCount*messageCount, func(ctx context.Context, msg *pubsub.Message) {
					delete(pending, string(msg.Message.Data))
					if len(pending) == 0 {
						wg.Done()
						logger.Info("all messages received")
					}
					logger.Debugw("received", "message", string(msg.Message.Data), "pending", len(pending))
				})

				subscription.Loop(t.Context(), destination.topic)
			})
		}()

		go func() {
			time.Sleep(1 * time.Second)
			iterator.ForEachIdx(nodes, func(i int, source *node) {
				logger := &utils.ZapLogger{SugaredLogger: logger.Named(fmt.Sprintf("source-%d", i))}
				for _, message := range source.messages {
					logger.Debugw("publishing", "message", message)
					require.NoError(t, source.topic.Publish(t.Context(), []byte(message), pubsub.WithReadiness(pubsub.MinTopicSize(1))))
					time.Sleep(throttledRate)
				}
			})
		}()

		wg.Wait()
	})

	t.Run("canceled context", func(t *testing.T) {
		logger, err := utils.NewZapLogger(utils.NewLogLevel(logLevel), true)
		require.NoError(t, err)

		node := getNode(t)
		ctx, cancel := context.WithCancel(t.Context())

		wg := conc.NewWaitGroup()
		wg.Go(func() {
			subscription := buffered.NewTopicSubscription(logger, nodeCount*messageCount, func(ctx context.Context, msg *pubsub.Message) {
				assert.Fail(t, "should not receive message")
			})

			subscription.Loop(ctx, node.topic)
		})

		cancel()
		wg.Wait()
	})
}

func getNode(t *testing.T) node {
	t.Helper()

	addr, err := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
	require.NoError(t, err)

	host, err := libp2p.New(
		libp2p.ListenAddrs(addr),
		libp2p.EnableRelayService(),
		libp2p.EnableHolePunching(),
		libp2p.NATPortMap(),
	)
	require.NoError(t, err)

	gossipSub, err := pubsub.NewGossipSub(t.Context(), host)
	require.NoError(t, err)

	topic, err := gossipSub.Join(topicName)
	require.NoError(t, err)

	// This is to make sure that the source hosts forward the messages
	_, err = topic.Subscribe(pubsub.WithBufferSize(nodeCount * messageCount))
	require.NoError(t, err)

	return node{
		host:     host,
		topic:    topic,
		messages: make([]string, messageCount),
	}
}

func connect(t *testing.T, source, destination host.Host) {
	t.Helper()
	peer := peer.AddrInfo{
		ID:    destination.ID(),
		Addrs: destination.Addrs(),
	}
	require.NoError(t, source.Connect(t.Context(), peer))
}
