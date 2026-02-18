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
	"github.com/NethermindEth/juno/p2p/pubsub/testutils"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/proto"
)

const (
	chainID      = "1"
	protocolID   = "test-vote-broadcasters-listeners-protocol"
	topicName    = "test-vote-broadcasters-listeners-topic"
	logLevel     = zapcore.PanicLevel
	voteCount    = 500
	invalidCount = 10
	maxWait      = 5 * time.Second
)

var network = &utils.Mainnet

func TestVoteBroadcastersAndListeners(t *testing.T) {
	logger, err := utils.NewZapLogger(utils.NewLogLevel(logLevel), true)
	require.NoError(t, err)

	prevotes := make([]starknet.Prevote, voteCount)
	precommits := make([]starknet.Precommit, voteCount)
	voteSet := make(map[string]struct{})

	for i := range voteCount {
		vote := getRandomVote(t)
		prevotes[i] = starknet.Prevote(vote)
		voteSet[getPrevoteString(&prevotes[i])] = struct{}{}

		vote = getRandomVote(t)
		precommits[i] = starknet.Precommit(vote)
		voteSet[getPrecommitString(&precommits[i])] = struct{}{}
	}

	pending := maps.Clone(voteSet)

	nodes := testutils.BuildNetworks(t, testutils.LineNetworkConfig(2))
	topics := nodes.JoinTopic(t, network, protocolID, topicName)

	source := topics[0]
	destination := topics[1]

	voteBroadcaster := vote.NewVoteBroadcaster(logger, vote.StarknetVoteAdapter, &config.DefaultBufferSizes)
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
			logger.Debug("broadcasting prevote", zap.String("vote", getPrevoteString(&prevotes[i])))
			prevoteBroadcaster.Broadcast(t.Context(), &prevotes[i])
		}
	}()

	// Broadcast the precommits in random order
	go func() {
		rand.Shuffle(voteCount, func(i, j int) {
			precommits[i], precommits[j] = precommits[j], precommits[i]
		})
		for i := range precommits {
			logger.Debug("broadcasting precommit", zap.String("vote", getPrecommitString(&precommits[i])))
			precommitBroadcaster.Broadcast(t.Context(), &precommits[i])
		}
	}()

	// Self broadcast invalid votes and make sure they are not processed
	go func() {
		for range invalidCount {
			voteBytes, err := proto.Marshal(&consensus.Vote{})
			require.NoError(t, err)
			_ = destination.Publish(t.Context(), voteBytes)
		}
	}()

	// Self broadcast non-votes and make sure they are not processed
	go func() {
		for range invalidCount {
			_ = destination.Publish(t.Context(), []byte("random"))
		}
	}()

	// Start the broadcaster loop
	go func() {
		voteBroadcaster.Loop(t.Context(), source)
	}()

	// Start the listener loop
	go func() {
		voteListeners.Loop(t.Context(), destination)
	}()

	for {
		select {
		case prevote := <-prevoteListener.Listen():
			voteStr := getPrevoteString(prevote)
			require.Contains(t, voteSet, voteStr)
			logger.Debug("received prevote", zap.String("vote", voteStr))
			delete(pending, voteStr)
		case precommit := <-precommitListener.Listen():
			voteStr := getPrecommitString(precommit)
			require.Contains(t, voteSet, voteStr)
			logger.Debug("received precommit", zap.String("vote", voteStr))
			delete(pending, voteStr)
		case <-time.After(maxWait):
			require.FailNow(t, "timeout")
		}

		logger.Info("remaining", zap.Int("pending", len(pending)))

		if len(pending) == 0 {
			return
		}
	}
}

func getRandomVote(t *testing.T) starknet.Vote {
	var id *starknet.Hash
	var err error
	if rand.IntN(100) >= 20 {
		id = felt.NewRandom[starknet.Hash]()
	}

	sender := felt.NewRandom[starknet.Address]()
	require.NoError(t, err)

	return starknet.Vote{
		MessageHeader: starknet.MessageHeader{
			Height: types.Height(rand.Uint64()),
			Round:  types.Round(rand.Int32()),
			Sender: *sender,
		},
		ID: id,
	}
}

func getPrevoteString(vote *starknet.Prevote) string {
	return getVoteString("prevote", (*starknet.Vote)(vote))
}

func getPrecommitString(vote *starknet.Precommit) string {
	return getVoteString("precommit", (*starknet.Vote)(vote))
}

func getVoteString(prefix string, vote *starknet.Vote) string {
	id := "nil"
	if vote.ID != nil {
		id = vote.ID.String()
	}

	return fmt.Sprintf("%v-%v-%v-%v-%v", prefix, vote.Height, vote.Round, vote.Sender, id)
}
