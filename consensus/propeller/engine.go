package propeller

import (
	"context"
	"fmt"
	"time"

	"github.com/NethermindEth/juno/utils/log"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

type broadcastResult struct {
	units []Unit
	errCh chan<- error
}

// todo(rdr): using String until I find a better type
type StakerID struct {
	peerID peer.ID       //nolint:unused // populated once committee key wiring lands.
	pubKey crypto.PubKey //nolint:unused // populated once committee key wiring lands.
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

func (registerCommittee) isCommand() {}

type unregisterCommittee struct {
	committeeID CommitteeID
}

func (unregisterCommittee) isCommand() {}

type broadcast struct {
	committeeID CommitteeID
	msg         []byte
	errCh       chan<- error
}

func (broadcast) isCommand() {}

type processUnit struct {
	unit   *Unit
	sender peer.ID
}

func (processUnit) isCommand() {}

// Engine is the central orchestrator of the Propeller protocol. It:
//
//   - Manages committee registrations (each committee has its own peer set and scheduler).
//   - Process all incoming messages and broadcasts them when expected.
//   - Handles broadcast requests from the service layer.
//   - Forwards all noteworthy event to the service layer.
type Engine struct {
	privKey   crypto.PrivKey
	localPeer peer.ID

	config Config
	logger log.StructuredLogger

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
	// todo(rdr): currently sent directly from the processor to the service,
	//            does the engine needs to do any filtering?
	// eventCh chan Event

	// cmdCh receives commands from the propeller service and act on those
	cmdCh chan engineCommand
}

// NewEngine creates an engine instance. It returns the engine and the channel to
// send engineCommands to.
// Call Run() to start processing.
//
// Parameters:
//   - privKey: this node's Ed25519 private key (for signing published messages).
//   - config: protocol parameters.
//   - log: structured logger.
//
// todo(rdr): Maybe in the future we don't want to expose the command channel and instead hide
// the interaction behind a public API. :think:
func NewEngine(
	privKey crypto.PrivKey,
	config *Config,
	logger log.StructuredLogger,
) (*Engine, chan<- engineCommand, <-chan Event) {
	localPeerID, err := peer.IDFromPrivateKey(privKey)
	if err != nil {
		// todo(rdr): pannic for now, error handling for later
		panic(err)
	}

	processor, eventsCh := NewProcessor(localPeerID, config)

	cmdCh := make(chan engineCommand)

	return &Engine{
		localPeer:     localPeerID,
		privKey:       privKey,
		config:        *config,
		logger:        logger,
		processor:     processor,
		committees:    make(map[CommitteeID]*committeeState),
		cmdCh:         cmdCh,
		unitsPrepared: make(chan broadcastResult),
		// Unsure of the fields below
		connectedPeers: make(map[peer.ID]struct{}),
	}, cmdCh, eventsCh
}

// registerCommittee creates the schedule and encoder for a new channel.
//
//nolint:unparam // peersKeys is part of the public registration API; wiring is still pending.
func (e *Engine) registerCommittee(
	committeeID *CommitteeID,
	peers []PeerCommittee,
	peersKeys []*StakerID,
) error {
	// todo(rdr): Why re-registration should be ignored,
	// as far as I understand, it shouldn't happen :think:
	if _, ok := e.committees[*committeeID]; ok {
		e.logger.Warn(
			"committee already registered, will ignore re-registration attempt",
			// todo(rdr): give a proper string repr
			zap.Any("committee id", committeeID),
		)
		return nil
	}

	schedule, err := NewScheduler(e.localPeer, peers)
	if err != nil {
		return fmt.Errorf("couldn't register a new committee: %w", err)
	}

	e.committees[*committeeID] = &committeeState{
		scheduler: schedule,
		// todo(rdr): need to add the peer pub keys
		peerKeys: nil,
	}

	e.logger.Info(
		"registered new committee",
		// todo(rdr): give a proper string representation
		zap.Any("committeeID", committeeID),
		zap.Int("peers", len(peers)),
		zap.Int("dataShards", schedule.NumDataShards()),
		zap.Int("codingShards", schedule.NumCodingShards()),
	)

	return nil
}

// unregisterCommittee removes a channel's state. Not new processors will be started but
// currently running ones will continue until the timeout / stop naturally
func (e *Engine) unregisterCommittee(committeeID *CommitteeID) {
	delete(e.committees, *committeeID)
	// todo(rdr): We have to  clean the processors, right?
	//            or will they shut down on their own eventually
	//            better to pass a context with cancel?

	e.logger.Info(
		"unregistered propeller committee",
		// todo(rdr): give a proper string representation
		zap.Any("committee id", committeeID),
	)
}

// prepareUnitsForBroadcast creates Proppeller units asynchronously since it is a very expensive
// operation.
func (e *Engine) prepareUnitsForBroadcast(
	committeeID *CommitteeID,
	data []byte,
	errCh chan<- error,
) error {
	cs, ok := e.committees[*committeeID]
	if !ok {
		return fmt.Errorf("cannot broadcast to an unregistered committee: %v", committeeID)
	}

	// todo(rdr): unsure if this approach of passing arguments to the go routine makes sense
	// todo(rdr): consider having a maximum amount of working threads and a queue tasks for this
	// This is an expensive operation, hence we need to do it separately
	go func(e *Engine, scheduler *Scheduler, committeeID CommitteeID, data []byte) {
		units, err := CreatePropellerUnits(
			e.privKey,
			&committeeID,
			// todo(rdr): Find how nonce is set when creating propeller units
			Nonce(time.Now().UnixNano()),
			data,
			scheduler.NumDataShards(),
			scheduler.NumCodingShards(),
		)
		if err != nil {
			errCh <- err
			return
		}

		// todo(rdr): Why do we send this back to the engine.Run thread instead of processing
		//           it right here?
		e.unitsPrepared <- broadcastResult{
			units: units,
			errCh: errCh,
		}
	}(e, cs.scheduler, *committeeID, data)

	return nil
}

// broacast receives Propeller units (built in `prepareBroadcast`) and sends them
//
//nolint:unparam // ctx will be used once the actual sending is wired up.
func (e *Engine) broadcast(ctx context.Context, units []Unit) error {
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
	// I need to pass to the eventCh all the units that it should receive

	return nil
}

// processUnit routes an incoming unit to the correct processor, creating
// one if needed.
func (e *Engine) processUnit(ctx context.Context, unit *Unit, sender peer.ID) {
	cs, ok := e.committees[unit.CommitteeID]
	if !ok {
		// note(rdr): maybe debug?
		e.logger.Warn(
			"received key for unregistered committee, dropping",
			// todo(rdr): give a proper string representation
			zap.Any("committee id", unit.CommitteeID),
		)
		return
	}

	err := e.processor.ProcessMessage(ctx, unit, sender, cs.scheduler)
	if err != nil {
		e.logger.Error("cannot process incoming unit", zap.Error(err))
	}
}

// handleCommand dispatches a command to the appropriate handler.
func (e *Engine) handleCommand(ctx context.Context, command engineCommand) {
	switch cmd := command.(type) {
	case *registerCommittee:
		err := e.registerCommittee(&cmd.committeeID, cmd.peers, cmd.peersKeys)
		cmd.errCh <- err
	case *unregisterCommittee:
		e.unregisterCommittee(&cmd.committeeID)
	case *broadcast:
		// we might need to pass the error channel here so that the internal go-routine
		// can forward it correctly (assuming a per command error channel)
		if err := e.prepareUnitsForBroadcast(&cmd.committeeID, cmd.msg, cmd.errCh); err != nil {
			cmd.errCh <- err
		}
	case *processUnit:
		e.processUnit(ctx, cmd.unit, cmd.sender)
	}
}

// Run starts the engine's main loop until context is cancelled.
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
			err := e.broadcast(ctx, broadcastResult.units)
			broadcastResult.errCh <- err
		}
	}
}

func (e *Engine) Broadcast(committeeID *CommitteeID, msg []byte) error {
	// todo(rdr): check how costly is this? Is there a better way than creating a channel
	errCh := make(chan error)
	e.cmdCh <- &broadcast{
		committeeID: *committeeID,
		msg:         msg,
		errCh:       errCh,
	}
	return <-errCh
}

func (e *Engine) RegisterCommittee(
	committeeID *CommitteeID,
	peers []PeerCommittee,
	// todo(rdr): peersKeys is something I don't know how to set correctly yet
	peersKeys []*StakerID,
) error {
	// todo(rdr): does creating an error channel per call is performant or
	//            should we have a pool of err channels or that is too crazy :3
	//            Thinking  on the GC cost...
	errCh := make(chan error)
	e.cmdCh <- &registerCommittee{
		committeeID: *committeeID,
		peers:       peers,
		peersKeys:   peersKeys,
		errCh:       errCh,
	}
	return <-errCh
}

func (e *Engine) UnregisterCommittee(committeeID *CommitteeID) {
	e.cmdCh <- &unregisterCommittee{committeeID: *committeeID}
}
