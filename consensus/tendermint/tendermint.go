package tendermint

import (
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/NethermindEth/juno/core/felt"
)

type (
	step        uint8
	height      uint
	round       uint
	votingPower uint
)

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
	maxFutureHeight = height(5)
	maxFutureRound  = round(5)
)

type timeoutFn func(r round) time.Duration

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
	Height() height

	// Commit is called by Tendermint when a block has been decided on and can be committed to the DB.
	Commit(height, V, []Precommit[H, A])
}

type Validators[A Addr] interface {
	// TotalVotingPower represents N which is required to calculate the thresholds.
	TotalVotingPower(height) votingPower

	// ValidatorVotingPower returns the voting power of the a single validator. This is also required to implement
	// various thresholds. The assumption is that a single validator cannot have voting power more than f.
	ValidatorVotingPower(A) votingPower

	// Proposer returns the proposer of the current round and height.
	Proposer(height, round) A
}

type Slasher[M Message[V, H, A], V Hashable[H], H Hash, A Addr] interface {
	// Equivocation informs the slasher that a validator has sent conflicting messages. Thus it can decide whether to
	// slash the validator and by how much.
	Equivocation(msgs ...M)
}

type Listener[M Message[V, H, A], V Hashable[H], H Hash, A Addr] interface {
	// Listen would return consensus messages to Tendermint which are set by the validator set.
	Listen() <-chan M
}

type Broadcaster[M Message[V, H, A], V Hashable[H], H Hash, A Addr] interface {
	// Broadcast will broadcast the message to the whole validator set. The function should not be blocking.
	Broadcast(M)

	// SendMsg would send a message to a specific validator. This would be required for helping send resquest and
	// response message to help a specifc validator to catch up.
	SendMsg(A, M)
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

	scheduledTms map[timeout]*time.Timer
	timeoutsCh   chan timeout

	// Future round messages are sent to the loop through the following channels
	proposalsCh  chan Proposal[V, H, A]
	prevotesCh   chan Prevote[H, A]
	precommitsCh chan Precommit[H, A]

	wg   sync.WaitGroup
	quit chan struct{}
}

type state[V Hashable[H], H Hash] struct {
	h height
	r round
	s step

	lockedValue *V
	lockedRound int
	validValue  *V
	validRound  int

	// The following are round level variable therefore when a round changes they must be reset.
	timeoutPrevoteScheduled       bool // line34 for the first time condition
	timeoutPrecommitScheduled     bool // line47 for the first time condition
	lockedValueAndOrValidValueSet bool // line36 for the first time condition
}

func New[V Hashable[H], H Hash, A Addr](nodeAddr A, app Application[V, H], chain Blockchain[V, H, A], vals Validators[A],
	listeners Listeners[V, H, A], broadcasters Broadcasters[V, H, A], tmPropose, tmPrevote, tmPrecommit timeoutFn,
) *Tendermint[V, H, A] {
	return &Tendermint[V, H, A]{
		nodeAddr: nodeAddr,
		state: state[V, H]{
			h:           chain.Height(),
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
		scheduledTms:     make(map[timeout]*time.Timer),
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
				delete(t.scheduledTms, tm)
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
		}
	}()
}

func (t *Tendermint[V, H, A]) Stop() {
	close(t.quit)
	t.wg.Wait()
	for _, tm := range t.scheduledTms {
		tm.Stop()
	}
}

func (t *Tendermint[V, H, A]) startRound(r round) {
	if r != 0 && r <= t.state.r {
		return
	}

	t.state.r = r
	t.state.s = propose

	t.state.timeoutPrevoteScheduled = false
	t.state.lockedValueAndOrValidValueSet = false
	t.state.timeoutPrecommitScheduled = false

	if p := t.validators.Proposer(t.state.h, r); p == t.nodeAddr {
		var proposalValue *V
		if t.state.validValue != nil {
			proposalValue = t.state.validValue
		} else {
			v := t.application.Value()
			proposalValue = &v
		}
		proposalMessage := Proposal[V, H, A]{
			H:          t.state.h,
			R:          r,
			ValidRound: t.state.validRound,
			Value:      proposalValue,
			Sender:     t.nodeAddr,
		}

		t.messages.addProposal(proposalMessage)
		t.broadcasters.ProposalBroadcaster.Broadcast(proposalMessage)
	} else {
		t.scheduleTimeout(t.timeoutPropose(r), propose, t.state.h, t.state.r)
	}

	go t.processFutureMessages(t.state.h, t.state.r)
}

//nolint:gocyclo
func (t *Tendermint[V, H, A]) processFutureMessages(h height, r round) {
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
	s step
	h height
	r round
}

func (t *Tendermint[V, H, A]) scheduleTimeout(duration time.Duration, s step, h height, r round) {
	tm := timeout{s: s, h: h, r: r}
	t.scheduledTms[tm] = time.AfterFunc(duration, func() {
		select {
		case <-t.quit:
		case t.timeoutsCh <- tm:
		}
	})
}

func (t *Tendermint[_, H, A]) OnTimeoutPropose(h height, r round) {
	if t.state.h == h && t.state.r == r && t.state.s == propose {
		vote := Prevote[H, A]{
			H:      t.state.h,
			R:      t.state.r,
			ID:     nil,
			Sender: t.nodeAddr,
		}
		t.messages.addPrevote(vote)
		t.broadcasters.PrevoteBroadcaster.Broadcast(vote)
		t.state.s = prevote
	}
}

func (t *Tendermint[_, H, A]) OnTimeoutPrevote(h height, r round) {
	if t.state.h == h && t.state.r == r && t.state.s == prevote {
		vote := Precommit[H, A]{
			H:      t.state.h,
			R:      t.state.r,
			ID:     nil,
			Sender: t.nodeAddr,
		}
		t.messages.addPrecommit(vote)
		t.broadcasters.PrecommitBroadcaster.Broadcast(vote)
		t.state.s = precommit
	}
}

func (t *Tendermint[_, _, _]) OnTimeoutPrecommit(h height, r round) {
	if t.state.h == h && t.state.r == r {
		t.startRound(r + 1)
	}
}

// line55 assumes the caller has acquired a mutex for accessing future messages.
/*
	55: upon f + 1 {∗, h_p, round, ∗, ∗} with round > round_p do
	56: 	StartRound(round)
*/
func (t *Tendermint[V, H, A]) line55(futureR round) {
	t.futureMessagesMu.Lock()

	vals := make(map[A]struct{})
	proposals, prevotes, precommits := t.futureMessages.allMessages(t.state.h, futureR)

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

	t.futureMessagesMu.Unlock()

	if t.validatorSetVotingPower(slices.Collect(maps.Keys(vals))) > f(t.validators.TotalVotingPower(t.state.h)) {
		t.startRound(futureR)
	}
}

func (t *Tendermint[V, H, A]) validatorSetVotingPower(vals []A) votingPower {
	var totalVotingPower votingPower
	for _, v := range vals {
		totalVotingPower += t.validators.ValidatorVotingPower(v)
	}
	return totalVotingPower
}

// Todo: add separate unit tests to check f and q thresholds.
func f(totalVotingPower votingPower) votingPower {
	// note: integer division automatically floors the result as it return the quotient.
	return (totalVotingPower - 1) / 3
}

func q(totalVotingPower votingPower) votingPower {
	// Unfortunately there is no ceiling function for integers in go.
	d := totalVotingPower * 2
	q := d / 3
	r := d % 3
	if r > 0 {
		q++
	}
	return q
}

func handleFutureRoundMessage[H Hash, A Addr, V Hashable[H], M Message[V, H, A]](
	t *Tendermint[V, H, A],
	m M,
	r func(M) round,
	addMessage func(M),
) bool {
	mR := r(m)
	if mR > t.state.r {
		if mR-t.state.r > maxFutureRound {
			return false
		}

		t.futureMessagesMu.Lock()
		addMessage(m)
		t.futureMessagesMu.Unlock()

		t.line55(mR)
		return false
	}
	return true
}

func handleFutureHeightMessage[H Hash, A Addr, V Hashable[H], M Message[V, H, A]](
	t *Tendermint[V, H, A],
	m M,
	h func(M) height,
	r func(M) round,
	addMessage func(M),
) bool {
	mH := h(m)
	mR := r(m)

	if mH > t.state.h {
		if mH-t.state.h > maxFutureHeight {
			return false
		}

		if mR > maxFutureRound {
			return false
		}

		t.futureMessagesMu.Lock()
		addMessage(m)
		t.futureMessagesMu.Unlock()
		return false
	}
	return true
}

func checkForQuorumPrecommit[H Hash, A Addr](precommitsForHR map[A][]Precommit[H, A], vID H) ([]Precommit[H, A], []A) {
	var precommits []Precommit[H, A]
	var vals []A

	for addr, valPrecommits := range precommitsForHR {
		for _, p := range valPrecommits {
			if *p.ID == vID {
				precommits = append(precommits, p)
				vals = append(vals, addr)
			}
		}
	}
	return precommits, vals
}

func checkForQuorumPrevotesGivenPrevote[H Hash, A Addr](p Prevote[H, A], prevotesForHR map[A][]Prevote[H, A]) []A {
	var vals []A
	for addr, valPrevotes := range prevotesForHR {
		for _, v := range valPrevotes {
			if *v.ID == *p.ID {
				vals = append(vals, addr)
			}
		}
	}
	return vals
}

func checkQuorumPrevotesGivenProposalVID[H Hash, A Addr](prevotesForHVr map[A][]Prevote[H, A], vID H) []A {
	var vals []A
	for addr, valPrevotes := range prevotesForHVr {
		for _, p := range valPrevotes {
			if *p.ID == vID {
				vals = append(vals, addr)
			}
		}
	}
	return vals
}

func (t *Tendermint[V, H, A]) checkForMatchingProposalGivenPrecommit(p Precommit[H, A],
	proposalsForHR map[A][]Proposal[V, H, A],
) *Proposal[V, H, A] {
	var proposal *Proposal[V, H, A]

	for _, prop := range proposalsForHR[t.validators.Proposer(p.H, p.R)] {
		if (*prop.Value).Hash() == *p.ID {
			propCopy := prop
			proposal = &propCopy
		}
	}
	return proposal
}

func (t *Tendermint[V, H, A]) checkForMatchingProposalGivenPrevote(p Prevote[H, A],
	proposalsForHR map[A][]Proposal[V, H, A],
) *Proposal[V, H, A] {
	var proposal *Proposal[V, H, A]

	for _, v := range proposalsForHR[t.validators.Proposer(p.H, p.R)] {
		if (*v.Value).Hash() == *p.ID {
			vCopy := v
			proposal = &vCopy
		}
	}
	return proposal
}
