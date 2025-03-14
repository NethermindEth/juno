package tendermint

import (
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/NethermindEth/juno/core/felt"
)

type step uint8

const (
	propose step = iota
	prevote
	precommit
)

func (s step) String() string {
	switch s {
	case propose:
		return "propose"
	case prevote:
		return "prevote"
	case precommit:
		return "precommit"
	default:
		return "unknown"
	}
}

const (
	maxFutureHeight = uint(5)
	maxFutureRound  = uint(5)
)

type timeoutFn func(round uint) time.Duration

type Addr interface {
	// Ethereum Addresses are 20 bytes
	~[20]byte | felt.Felt
}

type Hash interface {
	~[32]byte | felt.Felt
}

// Hashable's Hash() is used as ID()
type Hashable[H Hash] interface {
	Hash() H
}

type Application[V Hashable[H], H Hash] interface {
	// Value returns the value to the Tendermint consensus algorith which can be proposed to other validators.
	Value() V

	// Valid returns true if the provided value is valid according to the application context.
	Valid(V) bool
}

type Blockchain[V Hashable[H], H Hash, A Addr] interface {
	// Height return the current blockchain height
	Height() uint

	// Commit is called by Tendermint when a block has been decided on and can be committed to the DB.
	Commit(uint, V, []Precommit[H, A])
}

type Validators[A Addr] interface {
	// TotalVotingPower represents N which is required to calculate the thresholds.
	TotalVotingPower(height uint) uint

	// ValidatorVotingPower returns the voting power of the a single validator. This is also required to implement
	// various thresholds. The assumption is that a single validator cannot have voting power more than f.
	ValidatorVotingPower(validatorAddr A) uint

	// Proposer returns the proposer of the current round and height.
	Proposer(height, round uint) A
}

type Slasher[M Message[V, H, A], V Hashable[H], H Hash, A Addr] interface {
	// Equivocation informs the slasher that a validator has sent conflicting messages. Thus it can decide whether to
	// slash the validator and by how much.
	Equivocation(msgs ...M)
}

type Listener[M Message[V, H, A], V Hashable[H], H Hash, A Addr] interface {
	// Listen would return consensus messages to Tendermint which are set // by the validator set.
	Listen() <-chan M
}

type Broadcaster[M Message[V, H, A], V Hashable[H], H Hash, A Addr] interface {
	// Broadcast will broadcast the message to the whole validator set. The function should not be blocking.
	Broadcast(msg M)

	// SendMsg would send a message to a specific validator. This would be required for helping send resquest and
	// response message to help a specifc validator to catch up.
	SendMsg(validatorAddr A, msg M)
}

type Listeners[V Hashable[H], H Hash, A Addr] struct {
	ProposalListener  Listener[Proposal[V, H, A], V, H, A]
	PrevoteListener   Listener[Prevote[H, A], V, H, A]
	PrecommitListener Listener[Precommit[H, A], V, H, A]
}

type Broadcasters[V Hashable[H], H Hash, A Addr] struct {
	ProposalBroadcaster  Broadcaster[Proposal[V, H, A], V, H, A]
	PrevoteBroadcaster   Broadcaster[Prevote[H, A], V, H, A]
	PrecommitBroadcaster Broadcaster[Precommit[H, A], V, H, A]
}

type Tendermint[V Hashable[H], H Hash, A Addr] struct {
	nodeAddr A

	state state[V, H] // Todo: Does state need to be protected?

	messages       messages[V, H, A]
	futureMessages messages[V, H, A]

	futureMessagesMu *sync.Mutex

	timeoutPropose   timeoutFn
	timeoutPrevote   timeoutFn
	timeoutPrecommit timeoutFn

	application Application[V, H]
	blockchain  Blockchain[V, H, A]
	validators  Validators[A]

	listeners    Listeners[V, H, A]
	broadcasters Broadcasters[V, H, A]

	scheduledTms []timeout
	timeoutsCh   chan timeout

	// Future round messages are sent to the loop through the following channels
	proposalsCh  chan Proposal[V, H, A]
	prevotesCh   chan Prevote[H, A]
	precommitsCh chan Precommit[H, A]

	wg   sync.WaitGroup
	quit chan struct{}
}

type state[V Hashable[H], H Hash] struct {
	height uint
	round  uint
	step   step

	lockedValue *V
	lockedRound int
	validValue  *V
	validRound  int

	// The following are round level variable therefore when a round changes they must be reset.
	line34Executed bool
	line36Executed bool
	line47Executed bool
}

func New[V Hashable[H], H Hash, A Addr](addr A, app Application[V, H], chain Blockchain[V, H, A], vals Validators[A],
	listeners Listeners[V, H, A], broadcasters Broadcasters[V, H, A], tmPropose, tmPrevote, tmPrecommit timeoutFn,
) *Tendermint[V, H, A] {
	return &Tendermint[V, H, A]{
		nodeAddr: addr,
		state: state[V, H]{
			height:      chain.Height(),
			lockedRound: -1,
			validRound:  -1,
		},
		messages:         newMessages[V, H, A](),
		futureMessages:   newMessages[V, H, A](),
		futureMessagesMu: &sync.Mutex{},
		timeoutPropose:   tmPropose,
		timeoutPrevote:   tmPrevote,
		timeoutPrecommit: tmPrecommit,
		application:      app,
		blockchain:       chain,
		validators:       vals,
		listeners:        listeners,
		broadcasters:     broadcasters,
		scheduledTms:     make([]timeout, 0),
		timeoutsCh:       make(chan timeout),
		proposalsCh:      make(chan Proposal[V, H, A]),
		prevotesCh:       make(chan Prevote[H, A]),
		precommitsCh:     make(chan Precommit[H, A]),
		quit:             make(chan struct{}),
	}
}

func (t *Tendermint[V, H, A]) Start() {
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		t.startRound(0)

		// Todo: check message signature everytime a message is received.
		// For the time being it can be assumed the signature is correct.

		i := 0
		for {
			select {
			case <-t.quit:
				return
			case tm := <-t.timeoutsCh:
				// Handling of timeouts is priorities over messages
				switch tm.s {
				case propose:
					t.OnTimeoutPropose(tm.h, tm.r)
				case prevote:
					t.OnTimeoutPrevote(tm.h, tm.r)
				case precommit:
					t.OnTimeoutPrecommit(tm.h, tm.r)
				}

				i := slices.Index(t.scheduledTms, tm)
				t.scheduledTms = slices.Delete(t.scheduledTms, i, i+1)
			case p := <-t.proposalsCh:
				t.handleProposal(p)
			case p := <-t.listeners.ProposalListener.Listen():
				t.handleProposal(p)
			case p := <-t.prevotesCh:
				t.handlePrevote(p)
			case p := <-t.listeners.PrevoteListener.Listen():
				t.handlePrevote(p)
			case p := <-t.precommitsCh:
				t.handlePrecommit(p)
			case p := <-t.listeners.PrecommitListener.Listen():
				t.handlePrecommit(p)
			}
			i++
		}
	}()
}

func (t *Tendermint[V, H, A]) Stop() {
	close(t.quit)
	for _, tm := range t.scheduledTms {
		tm.Stop()
	}
	t.wg.Wait()
}

func (t *Tendermint[V, H, A]) startRound(r uint) {
	if r != 0 && r <= t.state.round {
		return
	}

	t.state.round = r
	t.state.step = propose

	t.state.line34Executed = false
	t.state.line36Executed = false
	t.state.line47Executed = false

	if p := t.validators.Proposer(t.state.height, r); p == t.nodeAddr {
		var proposalValue *V
		if t.state.validValue != nil {
			proposalValue = t.state.validValue
		} else {
			v := t.application.Value()
			proposalValue = &v
		}
		proposalMessage := Proposal[V, H, A]{
			Height:     t.state.height,
			Round:      r,
			ValidRound: t.state.validRound,
			Value:      proposalValue,
			Sender:     t.nodeAddr,
		}

		t.messages.addProposal(proposalMessage)
		t.broadcasters.ProposalBroadcaster.Broadcast(proposalMessage)
	} else {
		t.scheduleTimeout(t.timeoutPropose(r), propose, t.state.height, t.state.round)
	}

	go t.processFutureMessages(t.state.height, t.state.round)
}

//nolint:gocyclo
func (t *Tendermint[V, H, A]) processFutureMessages(h, r uint) {
	t.futureMessagesMu.Lock()
	defer t.futureMessagesMu.Unlock()

	proposals, prevotes, precommits := t.futureMessages.allMessages(h, r)
	if len(proposals) > 0 {
		for _, addrProposals := range proposals {
			for _, proposal := range addrProposals {
				select {
				case <-t.quit:
					return
				case t.proposalsCh <- proposal:
				}
			}
		}
	}

	if len(prevotes) > 0 {
		for _, addrPrevotes := range prevotes {
			for _, vote := range addrPrevotes {
				select {
				case <-t.quit:
					return
				case t.prevotesCh <- vote:
				}
			}
		}
	}

	if len(precommits) > 0 {
		for _, addrPrecommits := range precommits {
			for _, vote := range addrPrecommits {
				select {
				case <-t.quit:
					return
				case t.precommitsCh <- vote:

				}
			}
		}
	}

	t.futureMessages.deleteRoundMessages(h, r)
}

type timeout struct {
	*time.Timer

	s step
	h uint
	r uint
}

func (t *Tendermint[V, H, A]) scheduleTimeout(duration time.Duration, s step, h, r uint) {
	tm := timeout{s: s, h: h, r: r}
	tm.Timer = time.AfterFunc(duration, func() {
		select {
		case <-t.quit:
		case t.timeoutsCh <- tm:
		}
	})
	t.scheduledTms = append(t.scheduledTms, tm)
}

func (t *Tendermint[_, H, A]) OnTimeoutPropose(h, r uint) {
	if t.state.height == h && t.state.round == r && t.state.step == propose {
		vote := Prevote[H, A]{
			Vote: Vote[H, A]{
				Height: t.state.height,
				Round:  t.state.round,
				ID:     nil,
				Sender: t.nodeAddr,
			},
		}
		t.messages.addPrevote(vote)
		t.broadcasters.PrevoteBroadcaster.Broadcast(vote)
		t.state.step = prevote
	}
}

func (t *Tendermint[_, H, A]) OnTimeoutPrevote(h, r uint) {
	if t.state.height == h && t.state.round == r && t.state.step == prevote {
		vote := Precommit[H, A]{
			Vote: Vote[H, A]{
				Height: t.state.height,
				Round:  t.state.round,
				ID:     nil,
				Sender: t.nodeAddr,
			},
		}
		t.messages.addPrecommit(vote)
		t.broadcasters.PrecommitBroadcaster.Broadcast(vote)
		t.state.step = precommit
	}
}

func (t *Tendermint[_, _, _]) OnTimeoutPrecommit(h, r uint) {
	if t.state.height == h && t.state.round == r {
		t.startRound(r + 1)
	}
}

// line55 assumes the caller has acquired a mutex for accessing future messages.
func (t *Tendermint[V, H, A]) line55(futureR uint) {
	vals := make(map[A]struct{})
	proposals, prevotes, precommits := t.futureMessages.allMessages(t.state.height, futureR)

	// If a validator has sent proposl, prevote and precommit from a future round then it will only be counted once.
	for addr := range proposals {
		vals[addr] = struct{}{}
	}

	for addr := range prevotes {
		vals[addr] = struct{}{}
	}

	for addr := range precommits {
		vals[addr] = struct{}{}
	}

	if t.validatorSetVotingPower(slices.Collect(maps.Keys(vals))) > f(t.validators.TotalVotingPower(t.state.height)) {
		t.startRound(futureR)
	}
}

func (t *Tendermint[V, H, A]) validatorSetVotingPower(vals []A) uint {
	var totalVotingPower uint
	for _, v := range vals {
		totalVotingPower += t.validators.ValidatorVotingPower(v)
	}
	return totalVotingPower
}

// Todo: add separate unit tests to check f and q thresholds.
func f(totalVotingPower uint) uint {
	// note: integer division automatically floors the result as it return the quotient.
	return (totalVotingPower - 1) / 3
}

func q(totalVotingPower uint) uint {
	// Unfortunately there is no ceiling function for integers in go.
	d := totalVotingPower * 2
	q := d / 3
	r := d % 3
	if r > 0 {
		q++
	}
	return q
}
