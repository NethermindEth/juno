package validator

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/p2p/config"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/p2p/pubsub/testutils"
	"github.com/NethermindEth/juno/utils"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/sourcegraph/conc"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/proto"
)

const (
	logLevel      = zapcore.DebugLevel
	chainID       = "1"
	protocolID    = "proposal-stream-demux-test-protocol"
	topicName     = "proposal-stream-demux-test-topic"
	block0        = block1 - 1 // Block 1164618 uses block 1164617 as head block and 1164607 as revealed block
	block1        = 1164619    // Block 1164619 uses block 1164618 as head block and 1164608 as revealed block
	block2        = block1 + 1 // Block 1164620 uses block 1164619 as head block and 1164609 as revealed block
	block3        = block2 + 1 // Block 1164621 uses block 1164620 as head block and 1164610 as revealed block
	delay         = 10 * time.Millisecond
	firstHalfSize = 2
)

var network = &utils.Mainnet

func TestProposalStreamDemux(t *testing.T) {
	logger, err := utils.NewZapLogger(utils.NewLogLevel(logLevel), true)
	require.NoError(t, err)

	nodes := testutils.BuildNetworks(t, testutils.NewAdjacentNodes(1))
	topics := nodes.JoinTopic(t, network, protocolID, topicName)
	topic := topics[0]

	commitNotifier := make(chan types.Height)

	executor := NewMockExecutor(t, network)
	database := memory.New()
	bc := blockchain.New(database, network)
	builder := builder.New(bc, executor)
	transition := NewTransition(&builder, nil)
	proposalStore := proposal.ProposalStore[starknet.Hash]{}
	demux := NewProposalStreamDemux(logger, &proposalStore, transition, &config.DefaultBufferSizes, commitNotifier, block1)

	wg := conc.NewWaitGroup()
	wg.Go(func() {
		demux.Loop(t.Context(), topic)
	})
	t.Cleanup(func() {
		wg.Wait()
	})

	// Validate that old proposal is dropped
	messages0, _ := createProposal(t, executor, database, block0)
	// Validate that proposal sent in 2 halves is still captured
	messages1a, proposal1a := createProposal(t, executor, database, block1)
	// Validate that proposal sent in 1 sequence is captured
	messages1b, proposal1b := createProposal(t, executor, database, block1)
	// Validate that proposal not fully sent before commit is dropped
	messages1c, _ := createProposal(t, executor, database, block1)
	// Validate that proposal fully sent during the previous block is captured
	messages2a, proposal2a := createProposal(t, executor, database, block2)
	// Validate that proposal sent in 2 halves before and after previous block commit is still captured
	messages2b, proposal2b := createProposal(t, executor, database, block2)
	// Validate that proposal fully sent after previous block commit is captured
	messages2c, proposal2c := createProposal(t, executor, database, block2)
	// Validate that proposal fully sent several blocks ago is captured
	messages3, proposal3 := createProposal(t, executor, database, block3)

	outputs := demux.Listen()

	SetChainHeight(t, database, block0)
	t.Run("send full proposal 0", func(t *testing.T) {
		messages0 = sendParts(t, topic, messages0, len(messages0))
		assertNoExpectedProposal(t, outputs)
	})

	t.Run("send half of proposal 1a", func(t *testing.T) {
		messages1a = sendParts(t, topic, messages1a, firstHalfSize)
		assertNoExpectedProposal(t, outputs)
	})

	t.Run("send full proposal 1b", func(t *testing.T) {
		messages1b = sendParts(t, topic, messages1b, len(messages1b))
		assertExpectedProposal(t, outputs, &proposal1b)
	})

	t.Run("send full proposal 2a", func(t *testing.T) {
		messages2a = sendParts(t, topic, messages2a, len(messages2a))
		assertNoExpectedProposal(t, outputs)
	})

	t.Run("send full proposal 3", func(t *testing.T) {
		messages3 = sendParts(t, topic, messages3, len(messages3))
		assertNoExpectedProposal(t, outputs)
	})

	t.Run("send remaining of proposal 1a", func(t *testing.T) {
		messages1a = sendParts(t, topic, messages1a, len(messages1a))
		assertExpectedProposal(t, outputs, &proposal1a)
	})

	t.Run("send half proposal 2b", func(t *testing.T) {
		messages2b = sendParts(t, topic, messages2b, firstHalfSize)
		assertNoExpectedProposal(t, outputs)
	})

	t.Run("send half proposal 1c", func(t *testing.T) {
		messages1c = sendParts(t, topic, messages1c, firstHalfSize)
		assertNoExpectedProposal(t, outputs)
	})

	t.Run("commit block 1", func(t *testing.T) {
		SetChainHeight(t, database, block1)
		commitNotifier <- block1
		assertExpectedProposal(t, outputs, &proposal2a)
	})

	t.Run("send full proposal 2c", func(t *testing.T) {
		messages2c = sendParts(t, topic, messages2c, len(messages2c))
		assertExpectedProposal(t, outputs, &proposal2c)
	})

	t.Run("send remaining of proposal 2b", func(t *testing.T) {
		messages2b = sendParts(t, topic, messages2b, len(messages2b))
		assertExpectedProposal(t, outputs, &proposal2b)
	})

	t.Run("commit block 2", func(t *testing.T) {
		SetChainHeight(t, database, block2)
		commitNotifier <- block2
		assertExpectedProposal(t, outputs, &proposal3)
	})
}

func assertExpectedProposal(t *testing.T, outputs <-chan *starknet.Proposal, expectedProposal *starknet.Proposal) {
	select {
	case proposal := <-outputs:
		require.Equal(t, expectedProposal, proposal)
	case <-time.After(delay):
		require.Fail(t, "expected proposal")
	}
}

func assertNoExpectedProposal(t *testing.T, outputs <-chan *starknet.Proposal) {
	select {
	case <-outputs:
		require.Fail(t, "expected no proposal")
	case <-time.After(delay):
	}
}

func sendParts(t *testing.T, topic *pubsub.Topic, parts [][]byte, count int) [][]byte {
	for _, part := range parts[:count] {
		require.NoError(t, topic.Publish(t.Context(), part))
	}
	return parts[count:]
}

func createProposal(t *testing.T, executor *mockExecutor, database db.KeyValueStore, height types.Height) ([][]byte, starknet.Proposal) {
	testCase := TestCase{
		Height:     height,
		Round:      types.Round(rand.Uint32()),
		ValidRound: types.Round(rand.Uint32()),
		Network:    &utils.SepoliaIntegration,
	}
	fixture := NewEmptyTestFixture(t, executor, database, testCase)
	builder := newProposalBuilder(t)
	builder.addPart(fixture.ProposalInit)
	builder.addPart(fixture.ProposalCommitment)
	builder.addPart(fixture.ProposalFin)
	builder.addFin()

	return builder.messages, *fixture.Proposal
}

type proposalBuilder struct {
	t        *testing.T
	streamID string
	messages [][]byte
}

func newProposalBuilder(t *testing.T) *proposalBuilder {
	randomFelt := felt.NewRandom[felt.Felt]()

	return &proposalBuilder{
		t:        t,
		streamID: randomFelt.String(),
		messages: [][]byte{},
	}
}

func (b *proposalBuilder) addPart(part *consensus.ProposalPart) {
	partBytes, err := proto.Marshal(part)
	require.NoError(b.t, err)

	message := consensus.StreamMessage{
		StreamId:       []byte(b.streamID),
		SequenceNumber: uint64(len(b.messages)),
		Message: &consensus.StreamMessage_Content{
			Content: partBytes,
		},
	}

	b.addMessage(&message)
}

func (b *proposalBuilder) addFin() {
	b.addMessage(&consensus.StreamMessage{
		StreamId:       []byte(b.streamID),
		SequenceNumber: uint64(len(b.messages)),
		Message: &consensus.StreamMessage_Fin{
			Fin: &common.Fin{},
		},
	})
}

func (b *proposalBuilder) addMessage(message *consensus.StreamMessage) {
	messageBytes, err := proto.Marshal(message)
	require.NoError(b.t, err)

	b.messages = append(b.messages, messageBytes)
}
