package propeller

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

type unitWithSender struct {
	unit   *Unit
	sender peer.ID
}

type messageState uint64

const (
	preBuilt = iota
	preReceived
)

func (ms *messageState) NextState() {
	*ms += 1
}

type subprocessor struct {
	scheduler              *Scheduler
	localShardIndex        ShardIndex
	localShardWasBroadcast bool

	messageState  messageState
	unitsReceived []Unit
}

func newSubprocessor(scheduler *Scheduler, localShardIndex ShardIndex) subprocessor {
	return subprocessor{
		scheduler:              scheduler,
		localShardIndex:        localShardIndex,
		localShardWasBroadcast: false,

		messageState:  preBuilt,
		unitsReceived: make([]Unit, 0, scheduler.ReceiveThreshold()),
	}
}

func (s *subprocessor) Run(ctx context.Context, unitChan <-chan unitWithSender) error {
	for {
		select {
		case <-ctx.Done():
			// todo(rdr): need to differentiate between context cancellation and timeout.
			// can check for `context.DeadlineExceeded`
			return ctx.Err()
		case unitWithSender := <-unitChan:
			// todo(rdr): validate that the unit is correct
			// if the unit is incorrect penalize publisher (how?)

			s.unitsReceived = append(s.unitsReceived, *unitWithSender.unit)
			switch s.messageState {
			case preBuilt:
				// if the unit / shard is our own and we are pre-construction then we should
				// broadcast our own shard (only once)
				// todo(rdr): consider inlining this function? or use go naming ("once" in the name)
				s.maybeBroacastLocalShard(unitWithSender.unit)

				// todo(rdr): do something with a signature that I don't understand very well

				if len(s.unitsReceived) == s.scheduler.BuildThreshold() {
					s.messageState.NextState()
				}

			case preReceived:
				if len(s.unitsReceived) == s.scheduler.ReceiveThreshold() {
					// broadcast and finish execution – but don't broadcast the local shard
				}
			}

		}
	}
}

// todo(rdr): this can probably be inlined?
func (s *subprocessor) maybeBroacastLocalShard(unit *Unit) {
	if !s.localShardWasBroadcast && s.localShardIndex == unit.ShardIndex {
		// broadcast shard index
		s.localShardWasBroadcast = true
	}
}

// messageKey uniquely identifies a message within a committee. We track
// per-message state (processor, time cache) using this composite key
// because the same publisher could broadcast different messages (different
// roots) and we need to handle each independently.
type messageKey struct {
	CommitteeID CommitteeID
	Publisher   peer.ID
	Root        MessageRoot
}

type messageKeyWithError struct {
	messageKey messageKey
	error      error
}

type concurrentTasksBounds struct {
	maxWorkers             uint64
	maxWorkersPerPublisher uint64
}

// Processor handles all concurrent work on message processing
type Processor struct {
	finalized *TimeCache[messageKey]
	// todo(rdr): channel to communicate that a certain subprocessor has finished
	done chan messageKeyWithError

	mu             sync.Mutex
	publisherTasks map[peer.ID]uint64
	tasks          uint64
	// ----------------------------------
	subProcessors map[messageKey]chan unitWithSender

	// config inherited from Engine
	localPeer             peer.ID
	timeout               time.Duration
	concurrentTasksBounds concurrentTasksBounds
}

func NewProcessor(localPeer peer.ID, config *Config) *Processor {
	timeout := config.StaleMessageTimeout

	return &Processor{
		finalized: NewTimeCache[messageKey](timeout),
		done:      make(chan messageKeyWithError),

		publisherTasks: make(map[peer.ID]uint64),
		tasks:          0,
		subProcessors:  make(map[messageKey]chan unitWithSender),

		localPeer: localPeer,
		timeout:   timeout,
		// todo(rdr): set this ones based on the config (or some consts?)
		concurrentTasksBounds: concurrentTasksBounds{},
	}
}

func (p *Processor) Run(ctx context.Context) {
}

func (p *Processor) ProcessMessage(
	ctx context.Context,
	unit *Unit,
	sender peer.ID,
	scheduler *Scheduler,
) error {
	key := messageKey{
		CommitteeID: unit.CommitteeID,
		Publisher:   unit.Publisher,
		Root:        unit.MerkleRoot,
	}
	if p.finalized.Contains(key) {
		return nil
	}

	// todo(rdr): currently on a single go-routine the validation is performed and then the unit
	// is processed. This could be divided into:
	// - A validation task that performs validation (go routine A)
	// - A processing task that process the message (go routine B)
	// - Then A will send the correct units to B
	// This means that when many messages are received in quick succession, they can be validated
	// non blockingly. This also means we have two go routines by sub processor than just a single
	// one. Does it makes sense?
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

// createSubprocessor creates a go-routine (subprocessor) that handles all the processing of `key`.
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
		return nil, fmt.Errorf("cannot create new subprocessor: %w", err)
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
	// todo(rdr): should I pass p.done as an argument?
	go func(
		ctx context.Context,
		messageKey messageKey,
		scheduler *Scheduler,
		localShardIndex ShardIndex,
		unitChan <-chan unitWithSender,
	) {
		subProcessor := newSubprocessor(scheduler, localShardIndex)
		err := subProcessor.Run(ctx, unitChan)
		p.done <- messageKeyWithError{
			messageKey: messageKey,
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
) (chan unitWithSender, error) {
	unitChan, ok := p.subProcessors[*key]
	if !ok {
		return p.createSubprocessor(ctx, key, scheduler)
	}
	return unitChan, nil
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
