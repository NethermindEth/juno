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
	units []Unit
	err   error
}

// todo(rdr): using String until I find a better type
type StakerID struct {
	peerID peer.ID
	pubKey crypto.PubKey
}

// Holds the state for a Committee ID:
//   - The `scheduler` represents the Propeller Tree of peers
//   - The `processor` stores the state of this committee: when the built or receive threshold
//     have been reached
//   - The `peerKeys` I am not sure yet todo(rdr): <-
type committeeState struct {
	scheduler *Scheduler
	// todo(rdr): A look at processor shows that it's lifetime is strictly coupled with the
	//            state of a current committee. They both should be created and closed at the
	//            same time. If it is like this then it stands to reason that it should be coupled
	//            here. Not 100% sure right now, so leaving a big todo for now.

	// todo(rdr): why do we need this
	peerKeys map[peer.ID]crypto.PubKey
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

type unregisterCommittee struct {
	committeeID CommitteeID
}

func (unregisterCommittee) isCommand()

type broadcast struct {
	committeeID CommitteeID
	msg         []byte
	errCh       chan error
}

func (broadcast) isCommand()

type processUnit struct {
	unit   *Unit
	sender peer.ID
}

func (processUnit) isCommand()

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
	log       utils.StructuredLogger
	// processor handles validates and process all the messages received by other peers
	processor *Processor

	// committees holds the Scheduler (i.e. Propeller Tree) and Stakers ID of
	// the peers of each registered channel
	// todo(rdr): committeeState can set be there by value instead of by ref?
	committees map[CommitteeID]*committeeState

	// todo(rdr): not sure of this one yet
	// connected peers hold all the connected peers to the engine
	connectedPeers map[peer.ID]struct{}

	// whenever a broadcast action is started, units preparation are done concurrently
	// and delivered through this channel
	unitsPrepared chan broadcastResult

	// eventCh is shared between all processors and the engine. The engine
	// reads from it and forwards events to the application via Events().
	eventCh chan any

	// appEventCh is the externally-visible event channel. The engine copies
	// events from eventCh to appEventCh in its Run() loop, filtering out
	// internal events as needed.
	appEventCh chan any

	// cmdCh receives commands from the propeller service and act on those
	cmdCh <-chan engineCommand
}

// NewEngine creates an engine instance. It returns the engine and the channel to
// send engineCommands to.
// Call Run() to start processing.
//
// Parameters:
//   - localPeer: this node's peer ID.
//   - privKey: this node's Ed25519 private key (for signing published messages).
//   - config: protocol parameters.
//   - log: structured logger.
//
// todo(rdr): Maybe in the future we don't want to expose the command channel and instead hide
// the interaction behind a public API. :think:
func NewEngine(
	privKey crypto.PrivKey,
	config *Config,
	log utils.StructuredLogger,
) (*Engine, chan<- engineCommand) {
	localPeerID, err := peer.IDFromPrivateKey(privKey)
	if err != nil {
		// todo(rdr): pannic for now, error handling for later
		panic(err)
	}

	processor := NewProcessor(localPeerID, config)

	cmdCh := make(chan engineCommand)

	return &Engine{
		localPeer:     localPeerID,
		privKey:       privKey,
		config:        *config,
		log:           log,
		processor:     processor,
		committees:    make(map[CommitteeID]*committeeState),
		cmdCh:         cmdCh,
		unitsPrepared: make(chan broadcastResult),
		// Unsure of the fields below
		connectedPeers: make(map[peer.ID]struct{}),
		eventCh:        make(chan any, eventChSize),
		appEventCh:     make(chan any, appEventChSize),
	}, cmdCh
}

// registerCommittee creates the schedule and encoder for a new channel.
func (e *Engine) registerCommittee(
	committeeID CommitteeID,
	peers []PeerCommittee,
	peersKeys []*StakerID,
) error {
	// todo(rdr): Why re-registration should be ignored, as far as I know, it shouldn't happen :think:
	if _, ok := e.committees[committeeID]; ok {
		e.log.Warn(
			"committee already registered, will ignore re-registration attempt",
			zap.Uint64("committeeID", uint64(committeeID)),
		)
		return nil
	}

	// stakerIDs := make([]StakerID, len(peersKeys))
	// for i := range peersKeys {
	// 	if peersKeys[i] != nil {
	// 		stakerIDs[i] = *peersKeys[i]
	// 	} else {
	// 		// todo(rdr): re-check this flow once implementation is complete
	// 		panic("received nil key, they shoudln't be nil")
	// 	}
	// }

	schedule, err := NewScheduler(e.localPeer, peers)
	if err != nil {
		return fmt.Errorf("couldn't register a new committee: %w", err)
	}

	e.committees[committeeID] = &committeeState{
		scheduler: schedule,
		// todo(rdr): need to add the peer pub keys
		peerKeys: nil,
	}

	e.log.Info("registered new committee",
		zap.Uint64("committeeID", uint64(committeeID)),
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
	// todo(rdr): We have to  clean the processors, right?
	//            or will they shut down on their own eventually
	//            better to pass a context with cancelj

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
func (e *Engine) broadcast(units []Unit) error {
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

// processUnit routes an incoming unit to the correct processor, creating
// one if needed.
func (e *Engine) processUnit(ctx context.Context, unit *Unit, sender peer.ID) {
	cs, ok := e.committees[unit.CommitteeID]
	if !ok {
		// note(rdr): maybe debug?
		e.log.Warn("received key for unregistered committee, dropping",
			zap.Uint64("committee id", uint64(unit.CommitteeID)),
		)
		return
	}

	err := e.processor.ProcessMessage(ctx, unit, sender, cs.scheduler)
	if err != nil {
		e.log.Error("cannot process incoming unit", zap.Error(err))
	}
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
	case *unregisterCommittee:
		e.unregisterCommittee(cmd.committeeID)
	case *broadcast:
		err := e.prepareBroadcast(cmd.committeeID, cmd.msg)
		cmd.errCh <- err
	case *processUnit:
		e.processUnit(ctx, cmd.unit, cmd.sender)
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
