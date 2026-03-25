package propeller

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

// Channel buffer sizes for the engine's internal channels. These are large
// enough to absorb bursts without blocking, but bounded to prevent unbounded
// memory growth from slow consumers.
const (
	eventChSize    = 256
	cleanupChSize  = 256
	appEventChSize = 256
	cmdChSize      = 64
)

type broadcastResult struct {
	units []PropellerUnit
	err   error
}

// todo(rdr): using String until I find a better type
type StakerID string

type committeeState struct {
	scheduler *Scheduler
	peerKeys  []StakerID
}

// engineCommand is a tagged union of commands sent to the engine's Run() loop.
type engineCommand interface {
	isCommand()
}

type registerCommittee struct {
	committeeID CommitteeID
	peers       []PeerCommittee
	peersKeys   []*StakerID
	errCh       chan error
}

func (registerCommittee) isCommand()

type cmdUnregister struct {
	committeeID CommitteeID
}

func (cmdUnregister) isCommand()

type cmdBroadcast struct {
	committeeID CommitteeID
	msg         []byte
	errCh       chan error
}

func (cmdBroadcast) isCommand()

type cmdHandleUnit struct {
	unit   *PropellerUnit
	sender peer.ID
}

func (cmdHandleUnit) isCommand()

// Engine is the central orchestrator of the Propeller protocol. It:
//
//   - Manages channel registrations (each channel has its own peer set and schedule).
//   - Routes incoming PropellerUnits to the correct MessageProcessor.
//   - Handles broadcast requests from the application layer.
//   - Collects and forwards events from processors to the application.
//
// The engine is designed to be run as a single long-lived goroutine via Run().
// External callers interact with it through thread-safe methods that send
// commands on internal channels, so no locks are needed on the hot path.
type Engine struct {
	localPeer peer.ID
	privKey   crypto.PrivKey
	config    Config
	log       utils.Logger
	// committees holds the Scheduler (i.e. Propeller Tree) and Stakers ID of
	// the peers of each registered channel
	committees map[CommitteeID]*committeeState
	// connected peers hold all the connected peers to the engine
	connectedPeers map[peer.ID]struct{}

	// whenever a broadcast action is started, units preparaition are done concurrently
	// and delivered through this channel
	unitsPrepared chan broadcastResult

	// processors maps each active message to its processor's shard input
	// channel. The engine creates processors lazily on first shard receipt.
	// Only accessed from the Run() goroutine, so no lock needed.
	processors map[messageKey]chan<- shardDelivery

	// finalised tracks recently finalised messages to avoid re-creating
	// processors for late-arriving shards.
	finalised *TimeCache[messageKey]

	// eventCh is shared between all processors and the engine. The engine
	// reads from it and forwards events to the application via Events().
	eventCh chan any

	// cleanupCh carries internal processor-done signals. This is separate
	// from eventCh so that a full eventCh never blocks processor goroutines
	// trying to signal completion, which would leak goroutines.
	cleanupCh chan processorDone

	// appEventCh is the externally-visible event channel. The engine copies
	// events from eventCh to appEventCh in its Run() loop, filtering out
	// internal events as needed.
	appEventCh chan any

	// cmdCh carries commands from external callers into the Run() loop.
	cmdCh chan engineCommand

	// sendFn is the network callback for delivering units to peers.
	// Injected at construction time for testability.
	sendFn SendUnitFunc
}

// NewEngine creates an engine instance. Call Run() to start processing.
//
// Parameters:
//   - localPeer: this node's peer ID.
//   - privKey: this node's Ed25519 private key (for signing published messages).
//   - config: protocol parameters.
//   - sendFn: callback for delivering PropellerUnits to peers over the network.
//   - log: structured logger.
func NewEngine(
	// todo(rdr): this should be a key pair
	privKey crypto.PrivKey,
	config *Config,
	sendFn SendUnitFunc,
	log utils.Logger,
) *Engine {
	// todo(rdr): generate local peer id from keypair
	return &Engine{
		localPeer:      peer.ID("some random value for now"),
		privKey:        privKey,
		config:         *config,
		log:            log,
		committees:     make(map[CommitteeID]*committeeState),
		connectedPeers: make(map[peer.ID]struct{}),
		cmdCh:          make(chan engineCommand, cmdChSize),
		unitsPrepared:  make(chan broadcastResult),
		// Unsure of the fields below
		processors: make(map[messageKey]chan<- shardDelivery),
		finalised:  NewTimeCache[messageKey](config.StaleMessageTimeout * 2),
		eventCh:    make(chan any, eventChSize),
		cleanupCh:  make(chan processorDone, cleanupChSize),
		appEventCh: make(chan any, appEventChSize),
		sendFn:     sendFn,
	}
}

// registerCommittee creates the schedule and encoder for a new channel.
func (e *Engine) registerCommittee(
	committeeID CommitteeID,
	peers []PeerCommittee,
	peersKeys []*StakerID,
) error {
	if _, ok := e.committees[committeeID]; ok {
		e.log.Warn(
			"committee already registered, will ignore re-registration attempt",
			zap.Uint64("committeeID", uint64(committeeID)),
		)
		return nil
	}

	stakerIDs := make([]StakerID, len(peersKeys))
	for i := range peersKeys {
		if peersKeys[i] != nil {
			stakerIDs[i] = *peersKeys[i]
		} else {
			// todo(rdr): re-check this flow once implementation is complete
			panic("received nil key, they shoudln't be nil")
		}
	}

	schedule, err := NewScheduler(e.localPeer, peers)
	if err != nil {
		return fmt.Errorf("couldn't register a new committee: %w", err)
	}

	e.committees[committeeID] = &committeeState{
		scheduler: schedule,
		peerKeys:  stakerIDs,
	}

	e.log.Info("registered new committee",
		zap.Uint64("channel", uint64(committeeID)),
		zap.Int("peers", len(peers)),
		zap.Int("dataShards", schedule.NumDataShards()),
		zap.Int("codingShards", schedule.NumCodingShards()),
	)

	return nil
}

// unregisterCommittee removes a channel's state. Not new processors will be started but
// currently running ones will continue until the timeout / stop naturally
func (e *Engine) unregisterCommittee(committeeID CommitteeID) {
	delete(e.committees, committeeID)

	e.log.Info("unregistered propeller committee",
		zap.Uint64("committee", uint64(committeeID)),
	)
}

// prepareBroadcast creates Proppeller units asynchronously since it is a very expensive
// operation.
func (e *Engine) prepareBroadcast(committeeID CommitteeID, data []byte) error {
	cs, ok := e.committees[committeeID]
	if !ok {
		return fmt.Errorf("cannot broadcast to an unregistered committee: %s", committeeID)
	}

	// todo(rdr): consider having a maximum amount of working threads and a queue tasks for this
	// This is an expensive operation, hence we need to do it separately
	go func() {
		units, err := CreatePropellerUnits(
			committeeID,
			data,
			e.privKey,
			cs.scheduler.NumDataShards(),
			cs.scheduler.NumCodingShards(),
		)
		e.unitsPrepared <- broadcastResult{
			units: units,
			err:   err,
		}
	}()

	return nil
}

// broacast receives Propeller units (built in `prepareBroadcast`) and sends them
func (e *Engine) broadcast(units []PropellerUnit) error {
	targetCommittee := units[0].CommitteeID

	cs, ok := e.committees[targetCommittee]
	if !ok {
		return fmt.Errorf("target committee ID not found: %d", targetCommittee)
	}

	targetPeers := cs.scheduler.BroadcastTargets()
	if len(targetPeers) != len(units) {
		return fmt.Errorf(
			"different amount of target peers and propeller units to broadcast: %d vs %d",
			len(targetPeers),
			len(units),
		)
	}

	// todo(rdr): I need to do the actual sending

	return nil
}

// doHandleUnit routes an incoming unit to the correct processor, creating
// one if needed.
func (e *Engine) doHandleUnit(ctx context.Context, cmd *cmdHandleUnit) {
	unit := cmd.unit
	key := messageKey{
		Channel:   unit.CommitteeID,
		Publisher: unit.Publisher,
		Root:      unit.MerkleRoot,
	}

	// Skip already-finalised messages.
	if e.finalised.Contains(key) {
		return
	}

	// Route to existing processor or create a new one.
	shardCh, exists := e.processors[key]
	if !exists {
		shardCh = e.createProcessor(ctx, key, unit)
		if shardCh == nil {
			return // Channel not registered; logged inside createProcessor.
		}
	}

	// Non-blocking send to the processor. If its buffer is full, the shard
	// is dropped (the processor can reconstruct from other shards).
	select {
	case shardCh <- shardDelivery{Unit: unit, Sender: cmd.sender}:
	default:
		e.log.Warn("dropping shard: processor channel full",
			zap.Uint32("shard", uint32(unit.ShardIndex)),
			zap.Stringer("publisher", unit.Publisher),
		)
	}
}

// createProcessor spins up a new MessageProcessor goroutine for a message
// we haven't seen before.
func (e *Engine) createProcessor(
	ctx context.Context, key messageKey, unit *PropellerUnit,
) chan<- shardDelivery {
	cs, ok := e.committees[unit.CommitteeID]
	if !ok {
		e.log.Warn("received unit for unregistered channel",
			zap.Uint32("channel", uint32(unit.CommitteeID)),
		)
		return nil
	}

	validator := NewValidator(cs.scheduler, e.localPeer, &DefaultSignatureVerifier{})

	// Buffer the shard channel so the engine doesn't block when delivering
	// multiple shards in rapid succession.
	shardCh := make(chan shardDelivery, cs.scheduler.NumShards())

	proc := NewMessageProcessor(
		key.Channel,
		key.Publisher,
		key.Root,
		e.localPeer,
		e.config,
		cs.scheduler,
		validator,
		cs.encoder,
		shardCh,
		e.eventCh,
		e.sendFn,
	)

	e.processors[key] = shardCh

	// Launch the processor goroutine. It will run until finalisation,
	// timeout, or context cancellation. The cleanup signal goes to a
	// dedicated channel so it cannot be blocked by a full eventCh.
	go func() {
		proc.Run(ctx)
		select {
		case e.cleanupCh <- processorDone{key: key}:
		case <-ctx.Done():
		}
	}()

	return shardCh
}

// processorDone is an internal event signalling that a processor's goroutine
// has exited. The engine uses this to clean up the processors map.
type processorDone struct {
	key messageKey
}

// handleProcessorDone cleans up after a processor goroutine exits.
func (e *Engine) handleProcessorDone(done processorDone) {
	delete(e.processors, done.key)
	e.finalised.Add(done.key)

	// Periodically clean up expired entries in the time cache.
	// Amortised cost: we do it on every processor exit, which is
	// infrequent relative to shard processing.
	e.finalised.Cleanup()
}

// forwardEvent sends an event to the application's event channel. Non-blocking
// to avoid stalling the engine if the application is slow to consume events.
func (e *Engine) forwardEvent(event any) {
	select {
	case e.appEventCh <- event:
	default:
		e.log.Warn("dropping event: application event channel full")
	}
}

// handleCommand dispatches a command to the appropriate handler.
func (e *Engine) handleCommand(ctx context.Context, command engineCommand) {
	switch cmd := command.(type) {
	case *registerCommittee:
		err := e.registerCommittee(cmd.committeeID, cmd.peers, cmd.peersKeys)
		cmd.errCh <- err
	case *cmdUnregister:
		e.unregisterCommittee(cmd.committeeID)
	case *cmdBroadcast:
		err := e.prepareBroadcast(cmd.committeeID, cmd.msg)
		cmd.errCh <- err
	case *cmdHandleUnit:
		e.doHandleUnit(ctx, cmd)
	}
}

// Run starts the engine's main loop. It blocks until the context is cancelled.
// This should be called in its own goroutine.
//
// The loop processes three things concurrently:
//  1. Commands from external callers (register, broadcast, handle incoming unit).
//  2. Events from message processors (forward to application).
//  3. Context cancellation (graceful shutdown).
func (e *Engine) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case cmd := <-e.cmdCh:
			e.handleCommand(ctx, cmd)

		case broadcastResult := <-e.unitsPrepared:
			if broadcastResult.err != nil {
				e.log.Error("couldn't prepare units", zap.Error(broadcastResult.err))
			}
			e.broadcast(broadcastResult.units)

		case event := <-e.eventCh:
			// Forward application-visible events from processors.
			e.forwardEvent(event)

		case done := <-e.cleanupCh:
			// Processor goroutine exited; clean up the processors map.
			e.handleProcessorDone(done)
		}
	}
}

// Probably unuseful code

// Events returns the channel on which the application receives protocol
// events. The caller should read from this channel continuously to avoid
// backpressure on the engine.
// func (e *Engine) Events() <-chan any {
// 	return e.appEventCh
// }

// RegisterCommitee registers a committee with its peer set. This must be called
// before broadcasting on or receiving shards for a committee.
//
// The method blocks until the command is processed by the engine's Run() loop.
// todo(rdr): I am not sure this method should exist, or at least be defined at engine level
// func (e *Engine) RegisterCommittee(
// 	ctx context.Context,
// 	committeeID CommitteeID,
// 	peers []peer.ID,
// ) error {
// 	errCh := make(chan error, 1)
// 	select {
// 	case e.cmdCh <- &registerCommittee{
// 		committeeID: committeeID,
// 		peers:       peers,
// 		errCh:       errCh,
// 	}:
// 	case <-ctx.Done():
// 		return ctx.Err()
// 	}
//
// 	select {
// 	case err := <-errCh:
// 		return err
// 	case <-ctx.Done():
// 		return ctx.Err()
// 	}
// }

// // UnregisterCommittee removes a committee. Existing processors for that channel
// // will continue running until they finalise or time out, but no new
// // processors will be created.
// func (e *Engine) UnregisterCommittee(ctx context.Context, channel CommitteeID) error {
// 	select {
// 	case e.cmdCh <- &cmdUnregister{committeeID: channel}:
// 		return nil
// 	case <-ctx.Done():
// 		return ctx.Err()
// 	}
// }

// // Broadcast encodes and distributes a message to all peers in the given
// // channel. The local node acts as the publisher.
// //
// // The method blocks until the command is processed by the engine's Run() loop.
// func (e *Engine) Broadcast(
// 	ctx context.Context, channel CommitteeID, msg []byte,
// ) error {
// 	errCh := make(chan error, 1)
// 	select {
// 	case e.cmdCh <- &cmdBroadcast{
// 		channel: channel,
// 		msg:     msg,
// 		errCh:   errCh,
// 	}:
// 	case <-ctx.Done():
// 		return ctx.Err()
// 	}
//
// 	select {
// 	case err := <-errCh:
// 		return err
// 	case <-ctx.Done():
// 		return ctx.Err()
// 	}
// }

// // HandleUnit routes an incoming PropellerUnit from the network to the
// // appropriate message processor. This method is non-blocking: it sends
// // the unit to the engine's command channel.
// func (e *Engine) HandleUnit(unit *PropellerUnit, sender peer.ID) {
// 	// Non-blocking send: if the command channel is full, drop the unit.
// 	// This provides backpressure against flood attacks. The sender can
// 	// retry or the processor can reconstruct from other shards.
// 	select {
// 	case e.cmdCh <- &cmdHandleUnit{
// 		unit:   unit,
// 		sender: sender,
// 	}:
// 	default:
// 		e.log.Warn("dropping incoming unit: command channel full",
// 			zap.Uint32("shard", uint32(unit.ShardIndex)),
// 			zap.Stringer("publisher", unit.Publisher),
// 		)
// 	}
// }
