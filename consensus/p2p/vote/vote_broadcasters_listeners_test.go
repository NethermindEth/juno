package vote_test

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus/p2p/config"
	"github.com/NethermindEth/juno/consensus/p2p/vote"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/proto"
)

const (
	topicName     = "test-vote-broadcasters-listeners"
	bufferSize    = 1024
	logLevel      = zapcore.PanicLevel
	voteCount     = 1000
	invalidCount  = 100
	throttledRate = 5 * time.Millisecond
)

type node struct {
	host  host.Host
	topic *pubsub.Topic
}

func TestVoteBroadcastersAndListeners(t *testing.T) {
	logger, err := utils.NewZapLogger(utils.NewLogLevel(logLevel), true)
	require.NoError(t, err)

	prevotes := make([]starknet.Prevote, voteCount)
	precommits := make([]starknet.Precommit, voteCount)
	voteSet := make(map[string]struct{})

	for i := range voteCount {
		vote := getRandomVote(t)
		prevotes[i] = starknet.Prevote(vote)
		voteSet[getPrevoteString(prevotes[i])] = struct{}{}

		vote = getRandomVote(t)
		precommits[i] = starknet.Precommit(vote)
		voteSet[getPrecommitString(precommits[i])] = struct{}{}
	}

	pending := maps.Clone(voteSet)

	source := getNode(t)
	destination := getNode(t)

	connect(t, source.host, destination.host)

	voteBroadcaster := vote.NewVoteBroadcaster(logger, vote.StarknetVoteAdapter, bufferSize, config.DefaultBufferSizes.RetryInterval)
	prevoteBroadcaster := voteBroadcaster.AsPrevoteBroadcaster()
	precommitBroadcaster := voteBroadcaster.AsPrecommitBroadcaster()

	voteListeners := vote.NewVoteListeners[starknet.Value](logger, vote.StarknetVoteAdapter, &config.DefaultBufferSizes)
	prevoteListener := voteListeners.PrevoteListener
	precommitListener := voteListeners.PrecommitListener

	// Broadcast the prevotes in random order
	go func() {
		rand.Shuffle(voteCount, func(i, j int) {
			prevotes[i], prevotes[j] = prevotes[j], prevotes[i]
		})
		for i := range prevotes {
			logger.Debugw("broadcasting prevote", "vote", getPrevoteString(prevotes[i]))
			prevoteBroadcaster.Broadcast(prevotes[i])
			time.Sleep(throttledRate)
		}
	}()

	// Broadcast the precommits in random order
	go func() {
		rand.Shuffle(voteCount, func(i, j int) {
			precommits[i], precommits[j] = precommits[j], precommits[i]
		})
		for i := range precommits {
			logger.Debugw("broadcasting precommit", "vote", getPrecommitString(precommits[i]))
			precommitBroadcaster.Broadcast(precommits[i])
			time.Sleep(throttledRate)
		}
	}()

	// Self broadcast invalid votes and make sure they are not processed
	go func() {
		for range invalidCount {
			voteBytes, err := proto.Marshal(&consensus.Vote{})
			require.NoError(t, err)
			_ = destination.topic.Publish(t.Context(), voteBytes)
			time.Sleep(throttledRate)
		}
	}()

	// Self broadcast non-votes and make sure they are not processed
	go func() {
		for range invalidCount {
			_ = destination.topic.Publish(t.Context(), []byte("random"))
			time.Sleep(throttledRate)
		}
	}()

	// Start the broadcaster loop
	go func() {
		voteBroadcaster.Loop(t.Context(), source.topic)
	}()

	// Start the listener loop
	go func() {
		voteListeners.Loop(t.Context(), destination.topic)
	}()

	for {
		select {
		case prevote := <-prevoteListener.Listen():
			voteStr := getPrevoteString(prevote)
			require.Contains(t, voteSet, voteStr)
			logger.Debugw("received prevote", "vote", voteStr)
			delete(pending, voteStr)
		case precommit := <-precommitListener.Listen():
			voteStr := getPrecommitString(precommit)
			require.Contains(t, voteSet, voteStr)
			logger.Debugw("received precommit", "vote", voteStr)
			delete(pending, voteStr)
		}

		logger.Infow("remaining", "pending", len(pending))

		if len(pending) == 0 {
			return
		}
	}
}

func getRandomVote(t *testing.T) starknet.Vote {
	var id *starknet.Hash
	if rand.IntN(100) >= 20 {
		idFelt, err := new(felt.Felt).SetRandom()
		require.NoError(t, err)
		id = (*starknet.Hash)(idFelt)
	}

	sender, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	return starknet.Vote{
		MessageHeader: starknet.MessageHeader{
			Height: types.Height(rand.Uint64()),
			Round:  types.Round(rand.Int32()),
			Sender: starknet.Address(*sender),
		},
		ID: id,
	}
}

func getPrevoteString(vote starknet.Prevote) string {
	return getVoteString("prevote", starknet.Vote(vote))
}

func getPrecommitString(vote starknet.Precommit) string {
	return getVoteString("precommit", starknet.Vote(vote))
}

func getVoteString(prefix string, vote starknet.Vote) string {
	id := "nil"
	if vote.ID != nil {
		id = vote.ID.String()
	}

	return fmt.Sprintf("%v-%v-%v-%v-%v", prefix, vote.Height, vote.Round, vote.Sender, id)
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
	_, err = topic.Subscribe(pubsub.WithBufferSize(bufferSize))
	require.NoError(t, err)

	return node{
		host:  host,
		topic: topic,
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
