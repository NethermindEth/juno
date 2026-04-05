package propeller

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// processorState tracks which phase of the message lifecycle a processor is in.
// The transitions are strictly one-directional:
//
//	PreConstruction -> PostConstruction -> Finalised
//
// There is also a direct path from either state to Finalised via timeout.
type processorState int

const (
	// statePreConstruction: collecting shards, waiting to reach the build
	// threshold so we can reconstruct the original message.
	statePreConstruction processorState = iota

	// statePostConstruction: message has been reconstructed. We continue
	// counting incoming shards until we hit the receive threshold, which
	// guarantees that enough honest nodes have our shard to ensure all
	// other honest nodes can also reconstruct.
	statePostConstruction

	// stateFinalised: terminal state. The processor emits a result event
	// and stops accepting shards. The engine should clean up this processor.
	stateFinalised
)

// shardDelivery bundles an incoming shard with the peer that sent it,
// so the processor can validate the sender identity.
// todo(rdr): a better name for this
type shardDelivery struct {
	Unit   *Unit
	Sender peer.ID
}

// MessageProcessor manages the lifecycle of a single message identified by
// (channel, publisher, root). It runs as a goroutine that:
//
//  1. Accepts validated shards via its input channel.
//  2. In PreConstruction: collects shards until the build threshold is met,
//     then reconstructs the message via Reed-Solomon.
//  3. In PostConstruction: counts additional shards until the receive
//     threshold is met, then emits the message to the application.
//  4. On timeout: emits a timeout event and finalises.
//
// The processor is deliberately simple -- it owns no locks and communicates
// entirely through channels. All mutable state is confined to its goroutine.
type MessageProcessor struct {
	// Identity
	committeeID CommitteeID
	publisher   peer.ID
	root        MessageRoot

	// Config
	timeout time.Duration

	// Internal State.
	state             processorState
	shards            [][]byte // indexed by ShardIndex, nil = not yet received
	seenShards        map[ShardIndex]struct{}
	receivedCount     int
	signatureVerified bool
	storedSignature   []byte // cached from the first valid unit
	reconstructedMsg  []byte
	myShardUnit       *Unit // the unit we are responsible for forwarding

	// Channels.
	shardCh chan shardDelivery // incoming shards from the engine
	eventCh chan<- any         // outgoing events to the engine/application
}

// NewMessageProcessor creates a processor for a specific message. The caller
// must call Run() in a goroutine to start processing.
//
// Parameters:
//   - shardCh: the engine writes incoming shards here. Buffered to prevent
//     blocking the engine's main loop.
//   - eventCh: the processor writes lifecycle events here (shared with other
//     processors; the engine reads from it).
//   - sendFn: callback for network delivery of units to peers.
func NewMessageProcessor(
	channel CommitteeID,
	publisher peer.ID,
	root MessageRoot,
	localPeer peer.ID,
	config Config,
	schedule *Scheduler,
	validator *DeprecatedValidator,
	encoder Encoder,
	shardCh chan shardDelivery,
	eventCh chan<- any,
	sendFn SendUnitFunc,
) *MessageProcessor {
	return &MessageProcessor{
		committeeID: channel,
		publisher:   publisher,
		root:        root,
		localPeer:   localPeer,
		config:      config,
		schedule:    schedule,
		validator:   validator,
		encoder:     encoder,
		state:       statePreConstruction,
		shards:      make([][]byte, schedule.NumShards()),
		seenShards:  make(map[ShardIndex]bool),
		shardCh:     shardCh,
		eventCh:     eventCh,
		sendFn:      sendFn,
	}
}

// Run is the processor's main loop. It blocks until the processor finalises
// (either by completing the protocol or timing out) or the context is cancelled.
//
// The select on shardCh vs timer is the core of the state machine. We
// intentionally use a single goroutine to avoid any need for synchronisation
// on the processor's internal state.
func (p *MessageProcessor) Run(ctx context.Context) error {
	timer := time.NewTimer(p.timeout)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			if p.state != stateFinalised {
				p.emitEvent(EventMessageTimeout{
					Channel:   p.committeeID,
					Publisher: p.publisher,
					Root:      p.root,
				})
				p.state = stateFinalised
			}
			// throw an error processor is stopped after timeout?
			return nil

		case delivery, ok := <-p.shardCh:
			if !ok {
				// Channel closed by engine; processor is being shot down.
				return nil
			}
			if p.state == stateFinalised {
				return nil
			}
			p.handleShard(ctx, delivery)
			if p.state == stateFinalised {
				return nil
			}

		}
	}
}

// handleShard processes a single incoming shard delivery.
func (p *MessageProcessor) handleShard(ctx context.Context, delivery shardDelivery) {
	unit := delivery.Unit

	// Validate the unit.
	if err := p.validator.ValidateUnit(
		unit, delivery.Sender, p.seenShards, p.signatureVerified,
	); err != nil {
		p.emitEvent(EventShardValidationFailed{
			Sender:           delivery.Sender,
			ClaimedRoot:      unit.MerkleRoot,
			ClaimedPublisher: unit.Publisher,
			Err:              err,
		})
		return
	}

	// Mark the shard as received and store its data.
	p.seenShards[unit.ShardIndex] = true
	p.shards[unit.ShardIndex] = unit.ShardData
	p.receivedCount++

	// Cache the signature from the first valid unit. All units for the same
	// message carry the same publisher signature, so we only need one copy.
	// We store it here rather than in the unit slice because we only keep
	// shard data (not full units) to save memory.
	if !p.signatureVerified {
		p.storedSignature = make([]byte, len(unit.Signature))
		copy(p.storedSignature, unit.Signature)
	}
	p.signatureVerified = true

	switch p.state {
	case statePreConstruction:
		p.handlePreConstruction(ctx)
	case statePostConstruction:
		p.handlePostConstruction()
	case stateFinalised:
		// Should not reach here due to early return in Run, but be safe.
	}
}

// handlePreConstruction checks if we have enough shards to reconstruct.
func (p *MessageProcessor) handlePreConstruction(ctx context.Context) {
	if p.receivedCount < p.schedule.BuildThreshold() {
		return
	}

	// Attempt Reed-Solomon reconstruction.
	// We pass copies of the shard data because Reconstruct modifies the
	// slice in-place, and we don't want to corrupt our stored references.
	shardsCopy := make([][]byte, len(p.shards))
	for i, s := range p.shards {
		if s != nil {
			c := make([]byte, len(s))
			copy(c, s)
			shardsCopy[i] = c
		}
	}

	msg, err := ReconstructMessage(shardsCopy, p.schedule, p.encoder, p.root)
	if err != nil {
		p.emitEvent(EventReconstructionFailed{
			Root:      p.root,
			Publisher: p.publisher,
			Err:       err,
		})
		p.state = stateFinalised
		return
	}

	// Find our assigned shard so we can forward it to all other peers.
	myShard, err := p.schedule.ShardForPeer(p.publisher, p.localPeer)
	if err != nil {
		p.emitEvent(EventReconstructionFailed{
			Root:      p.root,
			Publisher: p.publisher,
			Err:       fmt.Errorf("determining my shard assignment: %w", err),
		})
		p.state = stateFinalised
		return
	}

	// Rebuild the Merkle tree from the complete shard set to get a valid
	// proof for our shard. We may not have received our own shard from
	// the network, so we need the proof from the reconstructed data.
	leaves := make([][]byte, len(shardsCopy))
	copy(leaves, shardsCopy)
	_, proofs := BuildMerkleTree(leaves)

	p.myShardUnit = &Unit{
		CommitteeID: p.committeeID,
		Publisher:   p.publisher,
		MerkleRoot:  p.root,
		Signature:   p.storedSignature,
		ShardIndex:  myShard,
		ShardData:   shardsCopy[myShard],
		MerkleProof: proofs[myShard],
	}

	p.reconstructedMsg = msg

	// Replace our sparse shard data with the fully reconstructed set.
	p.shards = shardsCopy

	// Count our own shard as held if we didn't receive it from the network.
	if !p.seenShards[myShard] {
		p.seenShards[myShard] = true
		p.receivedCount++
	}

	p.state = statePostConstruction

	// Broadcast our shard to all other peers (except the publisher, who
	// already has all shards).
	p.broadcastMyShard(ctx)

	// Check if we already meet the receive threshold (possible if many
	// shards arrived before reconstruction completed).
	p.handlePostConstruction()
}

// handlePostConstruction checks if the receive threshold has been met.
func (p *MessageProcessor) handlePostConstruction() {
	if p.receivedCount < p.schedule.ReceiveThreshold() {
		return
	}

	p.emitEvent(EventMessageReceived{
		Publisher: p.publisher,
		Root:      p.root,
		Message:   p.reconstructedMsg,
	})
	p.state = stateFinalised
}

// broadcastMyShard sends our assigned shard to all peers except the publisher.
// Failures are reported as events but do not stop the broadcast to other peers.
func (p *MessageProcessor) broadcastMyShard(ctx context.Context) {
	targets, err := p.schedule.BroadcastTargets(p.publisher)
	if err != nil {
		p.emitEvent(EventShardPublishFailed{
			Err: fmt.Errorf("getting broadcast targets: %w", err),
		})
		return
	}

	for _, target := range targets {
		if target == p.localPeer {
			// Don't send to ourselves.
			continue
		}

		if err := p.sendFn(ctx, target, p.myShardUnit); err != nil {
			p.emitEvent(EventShardSendFailed{
				From: p.localPeer,
				To:   target,
				Err:  err,
			})
		}
	}
}

// emitEvent sends an event to the application layer. Uses a non-blocking send
// so a slow consumer doesn't block the processor. The engine's event channel
// should be large enough that this rarely drops.
func (p *MessageProcessor) emitEvent(event any) {
	select {
	case p.eventCh <- event:
	default:
		// Event channel is full. This should be rare with a properly sized
		// buffer. The event is lost, but the processor continues operating.
	}
}
