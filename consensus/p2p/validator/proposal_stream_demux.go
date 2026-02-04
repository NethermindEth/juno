package validator

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/consensus/p2p/buffered"
	"github.com/NethermindEth/juno/consensus/p2p/config"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/conc/pool"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type streamID string

type ProposalStreamDemux[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	Loop(context.Context, *pubsub.Topic)
	Listen() <-chan *types.Proposal[V, H, A]
}

// proposalStreamDemux is a demux for the proposal streams.
//
// Motivation:
// In a consensus protocol, validators need to efficiently handle multiple concurrent proposal streams
// from different proposers across various consensus rounds and heights. Without proper demultiplexing,
// the system would struggle with:
// 1. Concurrent processing of proposals from multiple sources
// 2. Resource management and cleanup when heights advance
// 3. Proper isolation between different proposal streams
//
// This component solves these challenges by:
// - Initialising dedicated streams per proposal, running them concurrently and independently
// - Automatically managing stream lifecycle based on consensus height progression
// - Enabling concurrent proposal processing while maintaining safety guarantees
// - Centralising proposal routing logic to simplify downstream consumers
//
// It is responsible for:
// - Start the streams when a new height starts.
// - Demultiplex the incoming messages to the streams.
// - Cancel the current height streams when a new commit is received.
// Although the methods must be called sequentially and the streams are created, loaded, started, and stopped sequentially,
// the streams run concurrently.
type proposalStreamDemux struct {
	log                  utils.Logger
	proposalStore        *proposal.ProposalStore[starknet.Hash]
	transition           Transition
	bufferSizeConfig     *config.BufferSizes
	commitNotifier       <-chan types.Height
	streams              map[streamID]*proposalStream
	streamHeights        map[types.Height][]streamID
	outputs              chan *starknet.Proposal
	currentHeight        types.Height
	currentHeightCancel  context.CancelFunc
	currentHeightCtxPool *pool.ContextPool
}

func NewProposalStreamDemux(
	log utils.Logger,
	proposalStore *proposal.ProposalStore[starknet.Hash],
	transition Transition,
	bufferSizeConfig *config.BufferSizes,
	commitNotifier <-chan types.Height,
	currentHeight types.Height,
) ProposalStreamDemux[starknet.Value, starknet.Hash, starknet.Address] {
	return &proposalStreamDemux{
		log:              log,
		proposalStore:    proposalStore,
		transition:       transition,
		bufferSizeConfig: bufferSizeConfig,
		commitNotifier:   commitNotifier,
		streams:          make(map[streamID]*proposalStream),
		streamHeights:    make(map[types.Height][]streamID),
		outputs:          make(chan *starknet.Proposal, bufferSizeConfig.ProposalOutputs),
		currentHeight:    currentHeight,
	}
}

func (t *proposalStreamDemux) Loop(ctx context.Context, topic *pubsub.Topic) {
	defer close(t.outputs)

	messages := make(chan *pubsub.Message, t.bufferSizeConfig.ProposalDemux)
	defer close(messages)

	// Create a new pool for the current height
	t.createCtxPool(ctx)
	defer t.stop()

	wg := conc.NewWaitGroup()

	// Process the stream messages and the commits sequentially
	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case message := <-messages:
				if err := t.processStreamMessage(ctx, message); err != nil {
					t.log.Error("error processing stream message", zap.Error(err))
				}
			case height, ok := <-t.commitNotifier:
				if !ok {
					return
				}

				// Stop the current height pool and create a new one
				t.stop()
				t.createCtxPool(ctx)

				// Process the commit
				if err := t.processCommit(height); err != nil {
					t.log.Error("error processing commit", zap.Error(err))
				}
			}
		}
	})

	// Subscribe to the topic and forward messages to a channel
	wg.Go(func() {
		onMessage := func(ctx context.Context, message *pubsub.Message) {
			select {
			case <-ctx.Done():
				return
			case messages <- message:
			}
		}
		topicSubscription := buffered.NewTopicSubscription(t.log, t.bufferSizeConfig.ProposalDemux, onMessage)
		topicSubscription.Loop(ctx, topic)
	})

	wg.Wait()
}

func (t *proposalStreamDemux) Listen() <-chan *starknet.Proposal {
	return t.outputs
}

// stop stops the current height pool and deletes the streams for the current height
func (t *proposalStreamDemux) stop() {
	// First cancel all running stream of the current height
	t.currentHeightCancel()

	// Wait for all the streams to finish. Error of each stream is logged, so we ignore the combined error.
	_ = t.currentHeightCtxPool.Wait()

	// Close the input channels and delete the stream objects
	for _, streamID := range t.streamHeights[t.currentHeight] {
		t.streams[streamID].close()
		delete(t.streams, streamID)
	}

	// Finally delete the list of streams for the old height
	delete(t.streamHeights, t.currentHeight)
}

// createCtxPool creates a new pool for the current height
func (t *proposalStreamDemux) createCtxPool(ctx context.Context) {
	ctx, t.currentHeightCancel = context.WithCancel(ctx)
	t.currentHeightCtxPool = pool.New().WithContext(ctx)
}

// processCommit processes the commit for the current height
func (t *proposalStreamDemux) processCommit(height types.Height) error {
	if height != t.currentHeight {
		return fmt.Errorf("expected height %d, got %d", t.currentHeight+1, height)
	}

	// Start the all the streams for the new height
	t.currentHeight++
	for _, streamID := range t.streamHeights[t.currentHeight] {
		stream := t.streams[streamID]
		t.currentHeightCtxPool.Go(func(ctx context.Context) error {
			stream.loop(ctx)
			return nil
		})
	}

	return nil
}

// processStreamMessage processes the stream message.
func (t *proposalStreamDemux) processStreamMessage(ctx context.Context, pubsubMessage *pubsub.Message) error {
	message := consensus.StreamMessage{}
	if err := proto.Unmarshal(pubsubMessage.Data, &message); err != nil {
		return fmt.Errorf("unable to unmarshal stream message: %w", err)
	}

	streamID := streamID(message.StreamId)

	// If the msg is for this height, then start a go-routine
	// that will process this particular proposal in a loop
	if message.SequenceNumber == 0 {
		return t.onFirstMessage(ctx, streamID, &message)
	}

	// Forward the proposal onto the go-routine started above
	return t.onSubsequentMessage(ctx, streamID, &message)
}

// onFirstMessage is called when the first message is received for a stream.
func (t *proposalStreamDemux) onFirstMessage(ctx context.Context, streamID streamID, message *consensus.StreamMessage) error {
	stream := t.getStream(streamID)
	if stream.started {
		// If the stream is already started, then we don't need to start it again
		return nil
	}

	// Start the stream and get the height from the first message
	height, err := stream.start(ctx, message)
	if err != nil {
		return err
	}

	// If the height is less than the current height, the stream is outdated and should be deleted
	if height < t.currentHeight {
		return nil
	}

	// Add the stream to the list of streams for the height if height is greater or equal to the current height
	t.streamHeights[height] = append(t.streamHeights[height], streamID)

	// If the height is exactly the current height, start the stream loop
	if height == t.currentHeight {
		t.currentHeightCtxPool.Go(func(ctx context.Context) error {
			stream.loop(ctx)
			return nil
		})
	}

	return nil
}

// onSubsequentMessage is called when a subsequent message is received for a stream.
// The message is enqueued to the stream to be processed by the stream.
func (t *proposalStreamDemux) onSubsequentMessage(ctx context.Context, streamID streamID, message *consensus.StreamMessage) error {
	stream := t.getStream(streamID)
	stream.enqueueMessage(ctx, message)
	return nil
}

// getStream gets the stream for the given stream ID.
// If the stream does not exist, it creates a new one.
func (t *proposalStreamDemux) getStream(id streamID) *proposalStream {
	if stream, exists := t.streams[id]; exists {
		return stream
	}
	stream := newSingleProposalStream(t.log, t.proposalStore, t.transition, t.bufferSizeConfig.ProposalSingleStreamInput, t.outputs)
	t.streams[id] = stream
	return stream
}
