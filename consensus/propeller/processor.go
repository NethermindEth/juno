package propeller

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/NethermindEth/juno/consensus/propeller/timecache"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

type Event interface {
	isEvent()
}

type messageFinalized struct {
	message []byte
}

func (*messageFinalized) isEvent() {}

type broadcastUnit struct {
	unit  *Unit
	peers []peer.ID
}

func (*broadcastUnit) isEvent() {}

type broadcastMessage struct {
	unit []Unit
}

func (*broadcastMessage) isEvent() {}

type unitWithSender struct {
	unit   *Unit
	sender peer.ID
}

type subprocessor struct {
	scheduler       *Scheduler
	localPeer       peer.ID
	localShardIndex ShardIndex

	unitsChan        <-chan unitWithSender
	invalidUnitsChan chan<- invalidUnit
	processingEvents chan<- Event

	validator UnitValidator
}

func newSubprocessor(
	publisher peer.ID,
	scheduler *Scheduler,
	localPeer peer.ID,
	localShardIndex ShardIndex,
	unitsChan <-chan unitWithSender,
	invalidUnitsChan chan<- invalidUnit,
) subprocessor {
	return subprocessor{
		scheduler:       scheduler,
		localPeer:       localPeer,
		localShardIndex: localShardIndex,

		unitsChan:        unitsChan,
		invalidUnitsChan: invalidUnitsChan,

		validator: NewValidator(publisher, scheduler),
	}
}

func (s *subprocessor) broadcastUnit(unit *Unit) {
	index := 0
	peers := make([]peer.ID, len(s.scheduler.Peers())-2)
	for _, peerCommittee := range s.scheduler.Peers() {
		if peerCommittee.ID == unit.Publisher || peerCommittee.ID == s.localPeer {
			continue
		}
		// todo(rdr): index  out of range issue in this code
		peers[index] = peerCommittee.ID
		index += 1
	}
	rand.Shuffle(len(peers), func(i, j int) {
		peers[i], peers[j] = peers[j], peers[i]
	})

	s.processingEvents <- &broadcastUnit{
		unit:  unit,
		peers: peers,
	}
}

func (s *subprocessor) beforeMessageBuiltStage(ctx context.Context) (
	int, []byte, error,
) {
	// Keep track of the units received
	unitsReceived := make([]*Unit, s.scheduler.NumTotalShards())
	unitCount := 0

	localShardWasBroadcast := false

	// todo(rdr): we are triggering message building (expensive) as soon as the bulid threshold is
	// achieved, but it might be convenient to wait a few seconds to see if more messages
	// will arrive. Although, that will mean we also need to validate any of those extra messages.
	// The question is then: Do the cost of validating missing messages reduces greatly the cost
	// of recovering them? Cases to consider:
	//  - Perfect network condition: a lot of bandwidth and everybody is good. Does receiving all
	//     all the missing messages and validating them is cheaper than recovering them? What's the
	//     performance difference? <- Write benchmark
	//  - Bad network conditions: does the time waiting but receiving no messages will
	//     cause to waste a few seconds were the build was already done
	// -  Bad messages: the remaining messages we are waiting for and hence we incur on the cost
	//     of validating them but we get no benefit and we don't reduce the cost of recovering them.
	for unitCount != s.scheduler.BuildThreshold() {
		select {
		case <-ctx.Done():
			return 0, nil, ctx.Err()
		case unitWithSender := <-s.unitsChan:
			unit := unitWithSender.unit
			sender := unitWithSender.sender
			if err := s.validator.Validate(unit, sender); err != nil {
				s.invalidUnitsChan <- invalidUnit{
					// todo(rdr): not sure if we need message key.
					// We just want to penalize the sender
					messageKey: extractKey(unit),
					sender:     sender,
					error:      err,
				}
				// if this is the first unit we are receiving, finish abruptly since
				// it can be a DOS attack.
				if unitCount == 0 {
					return 0, nil, fmt.Errorf("couldn't validate first unit received: %w", err)
				}
				continue
			}

			unitsReceived[int(unit.ShardIndex)] = unit
			unitCount += 1

			// broadcast as soon as I get my shard
			if !localShardWasBroadcast && s.localShardIndex == unit.ShardIndex {
				localShardWasBroadcast = true
				s.broadcastUnit(unit)
			}
		}
	}

	fullMessage, localShardData, localProof, err := ConstructMessageFromUnits(
		unitsReceived,
		s.localShardIndex,
		s.scheduler.NumDataShards(),
		s.scheduler.NumCodingShards(),
	)
	if err != nil {
		return 0, nil, err
	}

	if !localShardWasBroadcast {
		// We pick a unit at random to fill the common data between the two. All of these values
		// have already been verified up top.
		// todo(rdr): there is an issue where unit in 0 is not guaranteed to be non-nil
		unit := unitsReceived[0]
		localUnit := Unit{
			CommitteeID: unit.CommitteeID,
			Publisher:   unit.Publisher,
			MessageRoot: unit.MessageRoot,
			Nonce:       unit.Nonce,
			Signature:   unit.Signature,
			MerkleProof: localProof,
			ShardIndex:  s.localShardIndex,
			ShardData:   localShardData,
		}
		s.broadcastUnit(&localUnit)
		unitCount += 1
	}

	return unitCount, fullMessage, nil
}

//nolint:unparam // message will be used once the receive-stage forwarding is wired up.
func (s *subprocessor) beforeMessageReceivedStage(
	ctx context.Context,
	unitCount int,
	message []byte,
) error {
	receiveThreshold := s.scheduler.ReceiveThreshold()
	for unitCount != receiveThreshold {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case unitWithSender := <-s.unitsChan:
			unit := unitWithSender.unit
			sender := unitWithSender.sender
			if err := s.validator.Validate(unit, sender); err != nil {
				s.invalidUnitsChan <- invalidUnit{
					messageKey: extractKey(unit),
					sender:     sender,
					error:      err,
				}
				continue
			}
			if unit.ShardIndex == s.localShardIndex {
				continue
			}
			unitCount += 1
		}
	}

	// todo(rdr): if we are here it means the message has been received.
	// forward it to (proc/engine/service <- one of these)

	return nil
}

// todo(rdr): we need to be sure to test both cases:
// - when built threshold == received threshold
// - when build threshold != received threshold
func (s *subprocessor) Run(
	ctx context.Context,
) error {
	// The Run function works in two main loops depending on the stage we are in.
	// First stage is before we can build the message, in which we receive messsages
	// until we have enough to build the full messsage. The local shard will be broadcasted
	// during this stage.
	// Second stage starts with the full message built and waits until we receive enough
	// messages to reach the received threshold, which guarantees that at least 2/3 of the
	// network is non faulty. Once there, Broadcast the rebuilt message and finishes

	unitCount, message, err := s.beforeMessageBuiltStage(ctx)
	if err != nil {
		return err
	}

	return s.beforeMessageReceivedStage(ctx, unitCount, message)
}

// messageKey are a copy of the values of a propeller unit that uniquely identifies it
// all unit that carries shard of the same message will have the same "key" fields
type messageKey struct {
	CommitteeID CommitteeID
	Publisher   peer.ID
	Root        MessageRoot
	Nonce       Nonce
}

func extractKey(unit *Unit) messageKey {
	return messageKey{
		CommitteeID: unit.CommitteeID,
		Publisher:   unit.Publisher,
		Root:        unit.MessageRoot,
		Nonce:       unit.Nonce,
	}
}

func (mk *messageKey) String() string {
	return fmt.Sprintf("%+v", *mk)
}

// invalidUnit is sent when a unit identified with `messageKey` failed validation with
// error `error`
type invalidUnit struct {
	messageKey messageKey
	sender     peer.ID
	error      error
}

// finalizedSubprocessor is sent once a subprocessor finalizes processing a message
// identified with `messageKey`. If it finalized on error the `error` field will be non-nil
type finalizedSubprocessor struct {
	messageKey messageKey
	error      error
}

type concurrentTasksBounds struct {
	maxWorkers             uint64
	maxWorkersPerPublisher uint64
}

// Processor handles all concurrent work on message processing
type Processor struct {
	// to avoid processing units already finalized
	finalized *timecache.TimeCache[messageKey]

	subProcessors map[messageKey]chan<- unitWithSender
	// channel through which subprocessors signal they have finalized execution
	subProcessorsFinalized chan finalizedSubprocessor
	// channel through which subprocessor share units that failed validation
	invalidUnits chan invalidUnit
	// channel through which important events are shared
	processingEvents chan<- Event

	// track current open and closed tasks to avoid resource starvation
	mu             sync.Mutex
	publisherTasks map[peer.ID]uint64
	tasks          uint64
	// config inherited from Engine
	localPeer             peer.ID
	timeout               time.Duration
	concurrentTasksBounds concurrentTasksBounds
	logger                log.StructuredLogger
}

// finalizedCacheSize bounds the number of recently-finalized message keys retained
// to avoid re-processing units belonging to messages already completed.
const finalizedCacheSize = 2048

func NewProcessor(localPeer peer.ID, config *Config) (*Processor, <-chan Event) {
	timeout := config.StaleMessageTimeout
	processingEvents := make(chan Event)

	return &Processor{
		finalized: timecache.New[messageKey](finalizedCacheSize, timeout),

		subProcessors:          make(map[messageKey]chan<- unitWithSender),
		subProcessorsFinalized: make(chan finalizedSubprocessor),
		invalidUnits:           make(chan invalidUnit),
		processingEvents:       processingEvents,

		mu:             sync.Mutex{},
		publisherTasks: make(map[peer.ID]uint64),
		tasks:          0,

		localPeer: localPeer,
		timeout:   timeout,
		// todo(rdr): set this ones based on the config (or some consts?)
		concurrentTasksBounds: concurrentTasksBounds{
			// dummy values for now
			maxWorkers:             1000,
			maxWorkersPerPublisher: 250,
		},
	}, processingEvents
}

func (p *Processor) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case finalizedSubP := <-p.subProcessorsFinalized:
			if finalizedSubP.error != nil {
				p.logger.Error(
					"subprocessor finalized with error",
					zap.String("message key", finalizedSubP.messageKey.String()),
					zap.Error(finalizedSubP.error),
				)
			} else {
				p.logger.Info(
					"subprocessor finalized",
					zap.String("message key", finalizedSubP.messageKey.String()),
				)
			}
			p.finalize(&finalizedSubP.messageKey)

		case invalidUnit := <-p.invalidUnits:
			p.logger.Error(
				"unit validation failed",
				zap.String("message key", invalidUnit.messageKey.String()),
				zap.Error(invalidUnit.error),
			)
			// todo(rdr): should we mark sender to penalize?
		}
	}
}

// ProcessMessage validates and process the received `unit` non-blockingly. It returns an
// error if the unit couldn't start processing.
func (p *Processor) ProcessMessage(
	ctx context.Context,
	unit *Unit,
	sender peer.ID,
	scheduler *Scheduler,
) error {
	key := extractKey(unit)
	if p.finalized.Get(&key) {
		return nil
	}

	// todo(rdr): currently on a single go-routine the validation is performed and then the unit
	// is processed. This could be divided into:
	// - A validation task that performs validation (go routine A)
	// - A processing task that process the message (go routine B)
	// - Then A will send the correct units to B
	// This means that when many messages are received in quick succession, they can be validated
	// non blockingly. This also means we have two go routines for sub processor rather than just
	// a single one.
	unitChan, err := p.subprocessorChannel(ctx, &key, scheduler)
	if err != nil {
		return fmt.Errorf("couldn't get processor channel for key: %w", err)
	}

	select {
	case unitChan <- unitWithSender{unit: unit, sender: sender}:
		return nil
	default:
	}

	return errors.New("dropping shard, processor channel full")
}

// createSubprocessor creates a go-routine (subprocessor) that handles all the processing of the
// messages identified with the given `messageKey`.
// It returns a channel through which this processor can be given units to process
// todo(rdr): I would like not to create a channel for everytime we have a different messageKey
// since that can be a bit rough to the GC, better to have  a pool of them. Benchmarks will give
// the final word
func (p *Processor) createSubprocessor(
	ctx context.Context,
	key *messageKey,
	scheduler *Scheduler,
) (chan<- unitWithSender, error) {
	localShardIndex, err := scheduler.ShardIndexForPublisher(key.Publisher)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get local shard index for publisher %s: %w", key.Publisher, err,
		)
	}

	err = p.increaseTasks(key.Publisher)
	if err != nil {
		return nil, err
	}

	// create communication channel
	unitChan := make(chan unitWithSender)
	p.subProcessors[*key] = unitChan

	// launch subprocessor
	ctxWithTimeout, cancel := context.WithTimeout(ctx, p.timeout)
	// todo(rdr): passing to avoid closures. Does it makes sense?
	// need to learn more how closures work in Go if it makes any difference
	// in performance.
	// todo(rdr): should I pass p.chan as an argument?
	go func(
		ctx context.Context,
		key messageKey,
		scheduler *Scheduler,
		localShardIndex ShardIndex,
		unitChan <-chan unitWithSender,
	) {
		defer cancel()
		subProcessor := newSubprocessor(
			key.Publisher, scheduler, p.localPeer, localShardIndex, unitChan, p.invalidUnits,
		)
		err := subProcessor.Run(ctx)
		p.subProcessorsFinalized <- finalizedSubprocessor{
			messageKey: key,
			error:      err,
		}
	}(ctxWithTimeout, *key, scheduler, localShardIndex, unitChan)

	return unitChan, nil
}

// Given a message key it returns a channel that communicates with the subprocessor
// handling this specific message key.
func (p *Processor) subprocessorChannel(
	ctx context.Context,
	key *messageKey,
	scheduler *Scheduler,
) (chan<- unitWithSender, error) {
	unitChan, ok := p.subProcessors[*key]
	if ok {
		return unitChan, nil
	}

	unitChan, err := p.createSubprocessor(ctx, key, scheduler)
	if err != nil {
		return nil, fmt.Errorf("creating new subprocessor: %w", err)
	}
	return unitChan, nil
}

func (p *Processor) finalize(key *messageKey) {
	p.decreaseTask(key.Publisher)
	delete(p.subProcessors, *key)
	p.finalized.Add(key)
}

func (p *Processor) increaseTasks(publisher peer.ID) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.publisherTasks[publisher] == p.concurrentTasksBounds.maxWorkersPerPublisher {
		return fmt.Errorf(
			"tasks per publisher exceeded (max: %d): %s",
			p.publisherTasks[publisher],
			publisher,
		)
	}

	if p.tasks == p.concurrentTasksBounds.maxWorkers {
		return fmt.Errorf(
			"max tasks that the processor can handle has been reached (max: %d)",
			p.tasks,
		)
	}

	p.publisherTasks[publisher] += 1
	p.tasks += 1

	return nil
}

func (p *Processor) decreaseTask(publisher peer.ID) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.publisherTasks[publisher] -= 1
	p.tasks -= 1
}
