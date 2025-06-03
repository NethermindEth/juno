package validator

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/consensus/p2p/buffered"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"google.golang.org/protobuf/proto"
)

type streamID string

type ProposalStreamDemux[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	Loop(context.Context)
	OnMessage(context.Context, *pubsub.Message)
	OnCommit(context.Context, types.Height)
	Listen() <-chan types.Proposal[V, H, A]
}

type proposalStreamDemux struct {
	log                    utils.Logger
	transition             Transition
	singleStreamBufferSize int
	streams                map[streamID]*proposalStream
	streamHeights          map[types.Height][]streamID
	currentHeight          types.Height
	messages               chan *pubsub.Message
	commits                chan types.Height
	outputs                chan starknet.Proposal
}

func NewProposalStreamDemux(
	log utils.Logger,
	transition Transition,
	currentHeight types.Height,
	bufferSizeConfig *buffered.BufferSizeConfig,
) ProposalStreamDemux[starknet.Value, starknet.Hash, starknet.Address] {
	return &proposalStreamDemux{
		log:                    log,
		transition:             transition,
		singleStreamBufferSize: bufferSizeConfig.ProposalSingleStreamInput,
		streams:                make(map[streamID]*proposalStream),
		streamHeights:          make(map[types.Height][]streamID),
		currentHeight:          currentHeight,
		messages:               make(chan *pubsub.Message, bufferSizeConfig.ProposalDemux),
		commits:                make(chan types.Height, bufferSizeConfig.ProposalCommitNotifier),
		outputs:                make(chan starknet.Proposal, bufferSizeConfig.ProposalOutputs),
	}
}

func (t *proposalStreamDemux) Loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case message := <-t.messages:
			if err := t.processStreamMessage(ctx, message); err != nil {
				t.log.Errorw("error processing stream message", "error", err)
			}
		case height := <-t.commits:
			if err := t.processCommit(ctx, height); err != nil {
				t.log.Errorw("error processing commit", "error", err)
			}
		}
	}
}

func (t *proposalStreamDemux) OnMessage(ctx context.Context, message *pubsub.Message) {
	select {
	case <-ctx.Done():
		return
	case t.messages <- message:
	}
}

func (t *proposalStreamDemux) OnCommit(ctx context.Context, height types.Height) {
	select {
	case <-ctx.Done():
		return
	case t.commits <- height:
	}
}

func (t *proposalStreamDemux) Listen() <-chan starknet.Proposal {
	return t.outputs
}

func (t *proposalStreamDemux) processCommit(ctx context.Context, height types.Height) error {
	if height != t.currentHeight {
		return fmt.Errorf("expected height %d, got %d", t.currentHeight+1, height)
	}
	// Close and delete the streams for the old height
	for _, streamID := range t.streamHeights[t.currentHeight] {
		t.streams[streamID].Close()
		delete(t.streams, streamID)
	}
	// Finally delete the list of streams for the old height
	delete(t.streamHeights, t.currentHeight)

	// Start the all the streams for the new height
	t.currentHeight++
	for _, streamID := range t.streamHeights[t.currentHeight] {
		go func(stream *proposalStream) {
			stream.Loop(ctx)
		}(t.streams[streamID])
	}

	return nil
}

func (t proposalStreamDemux) processStreamMessage(ctx context.Context, pubsubMessage *pubsub.Message) error {
	message := consensus.StreamMessage{}
	if err := proto.Unmarshal(pubsubMessage.Data, &message); err != nil {
		return fmt.Errorf("unable to unmarshal stream message: %w", err)
	}

	streamID := streamID(message.StreamId)

	if message.SequenceNumber == 0 {
		return t.onFirstMessage(ctx, streamID, &message)
	}

	return t.onSubsequentMessage(ctx, streamID, &message)
}

func (t *proposalStreamDemux) onFirstMessage(ctx context.Context, streamID streamID, message *consensus.StreamMessage) error {
	stream := t.getStream(streamID)
	// Start the stream and get the height from the first message
	height, err := stream.Start(ctx, message)
	if err != nil {
		return err
	}

	// If the height is less than the current height, the stream is outdated and should be deleted
	if height < t.currentHeight {
		delete(t.streams, streamID)
		return nil
	}

	// Add the stream to the list of streams for the height
	t.streamHeights[height] = append(t.streamHeights[height], streamID)

	// If the height is the current height, start the stream loop
	if height == t.currentHeight {
		go func() {
			stream.Loop(ctx)
		}()
	}

	return nil
}

func (t *proposalStreamDemux) onSubsequentMessage(ctx context.Context, streamID streamID, message *consensus.StreamMessage) error {
	stream := t.getStream(streamID)
	stream.Send(ctx, message)
	return nil
}

func (t *proposalStreamDemux) getStream(id streamID) *proposalStream {
	if stream, exists := t.streams[id]; exists {
		return stream
	}
	stream := newSingleProposalStream(t.log, t.transition, t.singleStreamBufferSize, t.outputs)
	t.streams[id] = stream
	return stream
}
