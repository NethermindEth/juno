package propeller

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

type unitWithSender struct {
	unit   *Unit
	sender peer.ID
}

type subprocessor struct {
	scheduler              *Scheduler
	localShardIndex        ShardIndex
	localShardWasBroadcast bool

	unitsChan        <-chan unitWithSender
	invalidUnitsChan chan<- invalidUnit

	validator Validator
}

func newSubprocessor(
	publisher peer.ID,
	scheduler *Scheduler,
	localShardIndex ShardIndex,
	unitsChan <-chan unitWithSender,
	invalidUnitsChan chan<- invalidUnit,
) subprocessor {
	return subprocessor{
		scheduler:       scheduler,
		localShardIndex: localShardIndex,

		unitsChan:        unitsChan,
		invalidUnitsChan: invalidUnitsChan,

		validator: NewValidator(publisher, scheduler),
	}
}

func (s *subprocessor) beforeMessageBuiltStage(ctx context.Context) (
	unitsReceived []*Unit,
	unitCount int,
	message []byte,
	err error,
) {
	// Keep track of the units received
	unitsReceived = make([]*Unit, s.scheduler.ReceiveThreshold())
	unitCount = 0

	localShardWasBroadcast := false

	buildThreshold := s.scheduler.BuildThreshold()
	for unitCount != buildThreshold {
		select {
		case <-ctx.Done():
			return
		case unitWithSender := <-s.unitsChan:
			unit := unitWithSender.unit
			sender := unitWithSender.sender

			err = s.validator.ValidateUnit(unit, sender)
			if err != nil {
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
					return
				}
				continue
			}

			unitsReceived[int(unit.ShardIndex)] = unit
			unitCount += 1

			// broadcast as soon as I get my shard
			if localShardWasBroadcast && s.localShardIndex == unit.ShardIndex {
				localShardWasBroadcast = true
				// todo(rdr): actually broadcast shard index
			}
		}
	}

	// perform the build thing
	panic("not implemented")
}

func (s *subprocessor) beforeMessageReceivedStage(
	ctx context.Context,
	unitsReceived []*Unit,
	unitCount int,
	message []byte,
) error {
	receivedThreshold := s.scheduler.ReceiveThreshold()
	for unitCount != receivedThreshold {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case unitWithSender := <-s.unitsChan:
			unit := unitWithSender.unit
			sender := unitWithSender.sender
			if err := s.validator.ValidateUnit(unit, sender); err != nil {
				s.invalidUnitsChan <- invalidUnit{
					messageKey: extractKey(unit),
					sender:     sender,
					error:      err,
				}
				continue
			}

			unitsReceived[int(unit.ShardIndex)] = unit
			unitCount += 1
		}
	}

	// do the actual job that requires doing once the receive threshold is reached
	panic("not implemented")
}

// todo(rdr): we need to be sure to test both cases:
// - when built threshold == received threshold
// - when build threshold != received threshold
func (s *subprocessor) Run(
	ctx context.Context,
) error {
	// The Run function works in two main loops depending on the stage we are in.
	// First stage is before we can build the message, where which we receive messsages
	// until we have enough to build the full messsage. The local shard will be broadcasted
	// during this stage.
	// Second stage starts with the full message built and waits until we receive enough
	// messages to reach the received threshold, which guarantees that at leasrt 2/3 of the
	// network is non faulty. This stages broadcasts the whole message once finished

	unitsReceived, unitCount, message, err := s.beforeMessageBuiltStage(ctx)
	if err != nil {
		return err
	}

	return s.beforeMessageReceivedStage(ctx, unitsReceived, unitCount, message)
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
	finalized *TimeCache[messageKey]

	subProcessors map[messageKey]chan unitWithSender
	// channel through wich subprocessors signal they have finalized execution
	subProcessorsFinalized chan finalizedSubprocessor
	// channel through which subprocessor sharedunits that failed validation
	invalidUnits chan invalidUnit

	// track current open and closed tasks to avoid resource starvation
	mu             sync.Mutex
	publisherTasks map[peer.ID]uint64
	tasks          uint64
	// config inherited from Engine
	localPeer             peer.ID
	timeout               time.Duration
	concurrentTasksBounds concurrentTasksBounds
	log                   utils.StructuredLogger
}

func NewProcessor(localPeer peer.ID, config *Config) *Processor {
	timeout := config.StaleMessageTimeout

	return &Processor{
		finalized: NewTimeCache[messageKey](timeout),

		subProcessors:          make(map[messageKey]chan unitWithSender),
		subProcessorsFinalized: make(chan finalizedSubprocessor),
		invalidUnits:           make(chan invalidUnit),

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
	}
}

func (p *Processor) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case finalizedSubP := <-p.subProcessorsFinalized:
			if finalizedSubP.error != nil {
				p.log.Error("subprocessor finalized with error",
					zap.String("message key", finalizedSubP.messageKey.String()),
					zap.Error(finalizedSubP.error),
				)
			} else {
				p.log.Info("subprocessor finalized",
					zap.String("message key", finalizedSubP.messageKey.String()),
				)
			}
			p.finalize(&finalizedSubP.messageKey)

		case invalidUnit := <-p.invalidUnits:
			p.log.Error("unit validation failed",
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
	if p.finalized.Contains(key) {
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
		fmt.Errorf("couldn't get processor channel for key: %w", err)
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
) (chan unitWithSender, error) {
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
	ctxWithTimeout, _ := context.WithTimeout(ctx, p.timeout)
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
		subProcessor := newSubprocessor(
			key.Publisher, scheduler, localShardIndex, unitChan, p.invalidUnits,
		)
		subProcessor.Run(ctx)
	}(ctxWithTimeout, *key, scheduler, localShardIndex, unitChan)

	return unitChan, nil
}

// Given a message key it returns a channel that communicates with the subprocessor
// handling this specific message key.
func (p *Processor) subprocessorChannel(
	ctx context.Context,
	key *messageKey,
	scheduler *Scheduler,
) (chan unitWithSender, error) {
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
	p.finalized.Add(*key)
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
