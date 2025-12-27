package buffered_test

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus/p2p/buffered"
	"github.com/NethermindEth/juno/consensus/p2p/config"
	"github.com/NethermindEth/juno/p2p/pubsub/testutils"
	"github.com/NethermindEth/juno/utils"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/conc/iter"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/proto"
)

const (
	chainID      = "1"
	protocolID   = "test-buffered-topic-subscription-protocol"
	topicName    = "test-buffered-topic-subscription-topic"
	nodeCount    = 20
	messageCount = 50
	logLevel     = zapcore.InfoLevel
	maxWait      = 30 * time.Second
)

type TestMessage = consensus.ConsensusStreamId

type origin struct {
	Source int
	Index  int
}

func TestBufferedTopicSubscriptionAndProtoBroadcaster(t *testing.T) {
	t.Run(fmt.Sprintf("%d nodes, each sending %d messages", nodeCount, messageCount), func(t *testing.T) {
		logger, err := utils.NewZapLogger(utils.NewLogLevel(logLevel), true)
		require.NoError(t, err)

		nodes := testutils.BuildNetworks(t, testutils.LineNetworkConfig(nodeCount))
		topics := nodes.JoinTopic(t, chainID, protocolID, topicName)

		messages := make([][]*TestMessage, nodeCount)
		allMessages := make(map[string]origin)

		for i := range messages {
			messages[i] = make([]*TestMessage, messageCount)
			for j := range messages[i] {
				msg := getTestMessage(i, j)
				messages[i][j] = msg

				msgBytes, err := proto.Marshal(msg)
				require.NoError(t, err)

				allMessages[string(msgBytes)] = origin{Source: i, Index: j}
			}
		}

		iterator := iter.Iterator[*pubsub.Topic]{MaxGoroutines: nodeCount}
		finished := make(chan struct{}, nodeCount)
		liveness := make(chan struct{}, 1)

		go func() {
			iterator.ForEachIdx(topics, func(i int, destination **pubsub.Topic) {
				logger := &utils.ZapLogger{
					Sugared: logger.Sugared.Named(fmt.Sprintf("destination-%d", i)),
				}
				pending := maps.Clone(allMessages)

				// Ignore the messages we are broadcasting
				for _, message := range messages[i] {
					msgBytes, err := proto.Marshal(message)
					require.NoError(t, err)
					delete(pending, string(msgBytes))
				}

				subscription := buffered.NewTopicSubscription(logger, nodeCount*messageCount, func(ctx context.Context, msg *pubsub.Message) {
					msgStr := string(msg.Message.Data)
					if _, ok := pending[msgStr]; !ok {
						return
					}

					select {
					case liveness <- struct{}{}:
					default:
					}

					delete(pending, msgStr)

					if len(pending) == 0 {
						finished <- struct{}{}
						logger.Info("all messages received")
					}
					logger.Debugw("received", "message", string(msg.Message.Data), "pending", len(pending))
				})

				subscription.Loop(t.Context(), *destination)
				if len(pending) > 0 {
					logger.Infow("missing messages", "pending", slices.Collect(maps.Values(pending)))
				}
			})
		}()

		go func() {
			iterator.ForEachIdx(topics, func(i int, source **pubsub.Topic) {
				logger := &utils.ZapLogger{SugaredLogger: logger.Named(fmt.Sprintf("source-%d", i))}
				rebroadcastInterval := config.DefaultBufferSizes.RebroadcastInterval

				var rebroadcastStrategy buffered.RebroadcastStrategy[*TestMessage]
				if i%2 == 0 {
					rebroadcastStrategy = buffered.NewRebroadcastStrategy(rebroadcastInterval, func(msg *TestMessage) uint64 {
						return msg.BlockNumber
					})
				}
				broadcaster := buffered.NewProtoBroadcaster(logger, messageCount, rebroadcastInterval, rebroadcastStrategy)
				go broadcaster.Loop(t.Context(), *source)
				for _, message := range messages[i] {
					logger.Debugw("publishing", "message", message)
					broadcaster.Broadcast(t.Context(), message)
				}
			})
		}()

		for range nodeCount {
			wait(t, liveness, finished)
		}
	})

	t.Run("canceled context", func(t *testing.T) {
		logger, err := utils.NewZapLogger(utils.NewLogLevel(logLevel), true)
		require.NoError(t, err)

		nodes := testutils.BuildNetworks(t, testutils.NewAdjacentNodes(1))
		topics := nodes.JoinTopic(t, chainID, protocolID, topicName)
		topic := topics[0]

		ctx, cancel := context.WithCancel(t.Context())

		wg := conc.NewWaitGroup()
		wg.Go(func() {
			subscription := buffered.NewTopicSubscription(logger, nodeCount*messageCount, func(ctx context.Context, msg *pubsub.Message) {
				assert.Fail(t, "should not receive message")
			})

			subscription.Loop(ctx, topic)
		})

		cancel()
		wg.Wait()
	})
}

func wait(t *testing.T, liveness, finished chan struct{}) {
	t.Helper()
	for {
		select {
		case <-finished:
			return
		case <-liveness:
			continue
		case <-time.After(maxWait):
			require.FailNow(t, "liveness check failed")
			return
		}
	}
}

func getTestMessage(node, messageIndex int) *TestMessage {
	return &TestMessage{
		Nonce: uint64(node*messageCount + messageIndex),
	}
}
