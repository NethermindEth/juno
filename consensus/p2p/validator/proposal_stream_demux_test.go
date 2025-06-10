package validator

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus/p2p/config"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/multiformats/go-multiaddr"
	"github.com/sourcegraph/conc"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/proto"
)

const (
	logLevel      = zapcore.DebugLevel
	topicName     = "proposal-stream-demux-test"
	block0        = block1 - 1
	block1        = 1337
	block2        = block1 + 1
	block3        = block2 + 1
	delay         = 10 * time.Millisecond
	txnPartCount  = 3
	firstHalfSize = 4
)

func TestProposalStreamDemux(t *testing.T) {
	logger, err := utils.NewZapLogger(utils.NewLogLevel(logLevel), true)
	require.NoError(t, err)

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

	commitNotifier := make(chan types.Height)

	demux := NewProposalStreamDemux(logger, NewTransition(), &config.DefaultBufferSizes, commitNotifier, block1)

	wg := conc.NewWaitGroup()
	wg.Go(func() {
		demux.Loop(t.Context(), topic)
	})
	t.Cleanup(func() {
		wg.Wait()
	})

	// Validate that old proposal is dropped
	messages0, _ := createProposal(t, block0)
	// Validate that proposal sent in 2 halves is still captured
	messages1a, proposal1a := createProposal(t, block1)
	// Validate that proposal sent in 1 sequence is captured
	messages1b, proposal1b := createProposal(t, block1)
	// Validate that proposal not fully sent before commit is dropped
	messages1c, _ := createProposal(t, block1)
	// Validate that proposal fully sent during the previous block is captured
	messages2a, proposal2a := createProposal(t, block2)
	// Validate that proposal sent in 2 halves before and after previous block commit is still captured
	messages2b, proposal2b := createProposal(t, block2)
	// Validate that proposal fully sent after previous block commit is captured
	messages2c, proposal2c := createProposal(t, block2)
	// Validate that proposal fully sent several blocks ago is captured
	messages3, proposal3 := createProposal(t, block3)

	outputs := demux.Listen()

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
		assertExpectedProposal(t, outputs, proposal1b)
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
		assertExpectedProposal(t, outputs, proposal1a)
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
		commitNotifier <- block1
		assertExpectedProposal(t, outputs, proposal2a)
	})

	t.Run("send full proposal 2c", func(t *testing.T) {
		messages2c = sendParts(t, topic, messages2c, len(messages2c))
		assertExpectedProposal(t, outputs, proposal2c)
	})

	t.Run("send remaining of proposal 2b", func(t *testing.T) {
		messages2b = sendParts(t, topic, messages2b, len(messages2b))
		assertExpectedProposal(t, outputs, proposal2b)
	})

	t.Run("commit block 2", func(t *testing.T) {
		commitNotifier <- block2
		assertExpectedProposal(t, outputs, proposal3)
	})
}

func assertExpectedProposal(t *testing.T, outputs <-chan starknet.Proposal, expectedProposal starknet.Proposal) {
	select {
	case proposal := <-outputs:
		require.Equal(t, expectedProposal, proposal)
	case <-time.After(delay):
		require.Fail(t, "expected proposal")
	}
}

func assertNoExpectedProposal(t *testing.T, outputs <-chan starknet.Proposal) {
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

func createProposal(t *testing.T, height types.Height) ([][]byte, starknet.Proposal) {
	proposer, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)
	proposerBytes := proposer.Bytes()

	round := rand.Uint32()
	value := uint64(time.Now().Unix())

	builder := newProposalBuilder(t)
	builder.addPart(&consensus.ProposalPart{
		Messages: &consensus.ProposalPart_Init{
			Init: &consensus.ProposalInit{
				BlockNumber: uint64(height),
				Round:       round,
				Proposer:    &common.Address{Elements: proposerBytes[:]},
			},
		},
	})

	builder.addPart(&consensus.ProposalPart{
		Messages: &consensus.ProposalPart_BlockInfo{
			BlockInfo: &consensus.BlockInfo{
				BlockNumber: uint64(height),
				Timestamp:   value,
			},
		},
	})

	for range txnPartCount {
		builder.addPart(&consensus.ProposalPart{
			Messages: &consensus.ProposalPart_Transactions{
				Transactions: &consensus.TransactionBatch{
					Transactions: []*consensus.ConsensusTransaction{},
				},
			},
		})
	}

	builder.addPart(&consensus.ProposalPart{
		Messages: &consensus.ProposalPart_Commitment{
			Commitment: &consensus.ProposalCommitment{
				BlockNumber: uint64(height),
				Timestamp:   value,
			},
		},
	})

	valueHash := felt.Felt(starknet.Value(value).Hash())
	valueHashBytes := valueHash.Bytes()

	builder.addPart(&consensus.ProposalPart{
		Messages: &consensus.ProposalPart_Fin{
			Fin: &consensus.ProposalFin{
				ProposalCommitment: &common.Hash{Elements: valueHashBytes[:]},
			},
		},
	})

	builder.addFin()

	proposal := starknet.Proposal{
		MessageHeader: starknet.MessageHeader{
			Height: height,
			Round:  types.Round(round),
			Sender: starknet.Address(*proposer),
		},
		Value:      (*starknet.Value)(&value),
		ValidRound: -1,
	}

	return builder.messages, proposal
}

type proposalBuilder struct {
	t        *testing.T
	streamID string
	messages [][]byte
}

func newProposalBuilder(t *testing.T) *proposalBuilder {
	randomFelt, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

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
